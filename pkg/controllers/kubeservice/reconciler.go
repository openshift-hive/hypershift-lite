package kubeservice

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hyperlitev1 "github.com/openshift-hive/hypershiftlite/pkg/api/v1alpha1"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/etcd"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/kas"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/kcm"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/ks"
	"github.com/openshift-hive/hypershiftlite/pkg/controllers/kubeservice/pki"
	"github.com/openshift-hive/hypershiftlite/pkg/releaseinfo"
	etcdv1 "github.com/openshift-hive/hypershiftlite/thirdparty/etcd/v1beta2"
)

const (
	defaultServiceCIDR = "172.30.0.0/16"
	defaultPodCIDR     = "10.128.0.0/14"
	etcdOperatorImage  = "quay.io/coreos/etcd-operator:v0.9.4"
	etcdVersion        = "3.4.9"
	kubeAPIServerPort  = 6443

	kubeAPIServerReplicas         = 1
	kubeControllerManagerReplicas = 1
	etcdClusterReplicas           = 1
)

type KubernetesServiceReconciler struct {
	client.Client
	Config *rest.Config

	recorder         record.EventRecorder
	releaseInfoCache map[string]*releaseinfo.ReleaseImage
	cacheMutex       sync.Mutex
}

func (r *KubernetesServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&hyperlitev1.KubernetesService{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 10*time.Second),
		}).
		Watches(&source.Kind{Type: &etcdv1.EtcdCluster{}}, &handler.EnqueueRequestForOwner{OwnerType: &hyperlitev1.KubernetesService{}}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{OwnerType: &hyperlitev1.KubernetesService{}}).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("kube-apiserver-controller")
	r.releaseInfoCache = map[string]*releaseinfo.ReleaseImage{}
	return nil
}

func (r *KubernetesServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("kubeService", req.NamespacedName.String())
	log.Info("Reconciling KubernetesService")
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the KubernetesService instance
	kubeService := &hyperlitev1.KubernetesService{}
	err := r.Client.Get(ctx, req.NamespacedName, kubeService)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Return early if deleted
	if !kubeService.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Reconcile etcd cluster status
	{
		log.Info("Reconciling Etcd status")
		etcdCluster := etcd.Cluster(req.Namespace)
		var err error
		if err = r.Get(ctx, types.NamespacedName{Namespace: etcdCluster.Namespace, Name: etcdCluster.Name}, etcdCluster); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to fetch etcd cluster %s/%s: %w", etcdCluster.Namespace, etcdCluster.Name, err)
		}
		if apierrors.IsNotFound(err) {
			log.Info("Etcd cluster does not exist yet")
			etcdCluster = nil
		} else {
			if !etcdCluster.DeletionTimestamp.IsZero() {
				// Wait til etcd cluster is gone in case it's being deleted
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}
		err = etcd.ReconcileEtcdClusterStatus(ctx, r.Client, kubeService, etcdCluster)
		if err != nil {
			log.Error(err, "etcd status reconcile failed")
			return ctrl.Result{}, err
		}
	}
	// Reconcile kas status
	{
		log.Info("Reconciling Kube APIServer status")
		kasDeployment := kas.Deployment(req.Namespace)
		var err error
		if err = r.Get(ctx, types.NamespacedName{Namespace: kasDeployment.Namespace, Name: kasDeployment.Name}, kasDeployment); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to fetch kas deployment %s/%s: %w", kasDeployment.Namespace, kasDeployment.Name, err)
		}
		if apierrors.IsNotFound(err) {
			log.Info("Kube API server deployment does not exist yet")
			kasDeployment = nil
		} else if !kasDeployment.DeletionTimestamp.IsZero() {
			// Wait til deployment is gone in case it's being deleted
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		err = kas.ReconcileKubeAPIServerDeploymentStatus(ctx, r.Client, kubeService, kasDeployment)
		if err != nil {
			log.Error(err, "kube apiserver status reconcile failed")
		}
	}
	// Reconcile kcm status
	{
		log.Info("Reconciling Kube controller manager status")
		kcmDeployment := kcm.Deployment(req.Namespace)
		var err error
		if err = r.Get(ctx, types.NamespacedName{Namespace: kcmDeployment.Namespace, Name: kcmDeployment.Name}, kcmDeployment); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to fetch kcm deployment %s/%s: %w", kcmDeployment.Namespace, kcmDeployment.Name, err)
		}
		if apierrors.IsNotFound(err) {
			log.Info("Kube controller manager deployment does not exist yet")
			kcmDeployment = nil
		} else if !kcmDeployment.DeletionTimestamp.IsZero() {
			// Wait til deployment is gone in case it's being deleted
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		err = kcm.ReconcileKubeControllerManagerDeploymentStatus(ctx, r.Client, kubeService, kcmDeployment)
		if err != nil {
			log.Error(err, "kube apiserver status reconcile failed")
			return ctrl.Result{}, err
		}
	}
	// Reconcile ks status
	{

		etcdAvailable := ks.GetConditionByType(kubeService.Status.Conditions, hyperlitev1.EtcdAvailable)
		kasAvailable := ks.GetConditionByType(kubeService.Status.Conditions, hyperlitev1.KubeAPIServerAvailable)
		kcmAvailable := ks.GetConditionByType(kubeService.Status.Conditions, hyperlitev1.KubeControllerManagerAvailable)

		if etcdAvailable != nil && kasAvailable != nil && kcmAvailable != nil &&
			etcdAvailable.Status == corev1.ConditionTrue &&
			kasAvailable.Status == corev1.ConditionTrue &&
			kcmAvailable.Status == corev1.ConditionTrue {
			ks.SetConditionByType(&kubeService.Status.Conditions, hyperlitev1.Available, corev1.ConditionTrue, "Running", "Kubernetes service is up and running")
		} else {
			ks.SetConditionByType(&kubeService.Status.Conditions, hyperlitev1.Available, corev1.ConditionFalse, "NotAvailable", "Kubernetes service is not yet available")
		}
		if err := r.Status().Update(ctx, kubeService); err != nil {
			log.Error(err, "failed to update kubernetes service status")
			return ctrl.Result{}, err
		}
	}

	// Reconcile root CA
	rootCASecret := pki.RootCASecret(kubeService.Namespace)
	if _, err = controllerutil.CreateOrUpdate(ctx, r, rootCASecret, func() error {
		ensureKSOwnerRef(kubeService, rootCASecret)
		return pki.ReconcileRootCA(rootCASecret)
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile root CA: %w", err)
	}

	log.Info("Reconciling Etcd")
	err = r.reconcileEtcd(ctx, kubeService)
	if err != nil {
		log.Error(err, "failed to reconcile etcd")
		return ctrl.Result{}, err
	}
	{
		etcdAvailable := ks.GetConditionByType(kubeService.Status.Conditions, hyperlitev1.EtcdAvailable)
		if etcdAvailable == nil || etcdAvailable.Status != corev1.ConditionTrue {
			log.Info("etcd is not yet available")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// Get release image info
	releaseImage, err := r.getReleaseImage(ctx, req.Namespace, kubeService.Spec.ReleaseImage, kubeService.Spec.PullSecret.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile K8s API server
	log.Info("Reconciling Kube API server")
	err = r.reconcileKubeAPIServer(ctx, kubeService, releaseImage)
	if err != nil {
		log.Error(err, "failed to reconcile kube api server")
		return ctrl.Result{}, err
	}
	{
		kasAvailable := ks.GetConditionByType(kubeService.Status.Conditions, hyperlitev1.KubeAPIServerAvailable)
		if kasAvailable == nil || kasAvailable.Status != corev1.ConditionTrue {
			log.Info("kube APIServer is not yet available")
			return ctrl.Result{}, nil
		}
	}

	// Reconcile Kube controller manager
	log.Info("Reconciling Kube Controller Manager")
	err = r.reconcileKubeControllerManager(ctx, kubeService, releaseImage)
	if err != nil {
		log.Error(err, "failed to reconcile kube controller manager")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed")
	return ctrl.Result{}, nil
}

func (r *KubernetesServiceReconciler) getReleaseImage(ctx context.Context, namespace, imagePullSpec, pullSecretName string) (*releaseinfo.ReleaseImage, error) {
	log := ctrl.LoggerFrom(ctx)
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()
	if img, exists := r.releaseInfoCache[imagePullSpec]; exists {
		return img, nil
	}

	kubeClient, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		log.Error(err, "unable to create kube client")
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	releaseInfoProvider := &releaseinfo.PodProvider{
		Pods: kubeClient.CoreV1().Pods(namespace),
	}
	img, err := releaseInfoProvider.Lookup(ctx, imagePullSpec, pullSecretName)
	if err != nil {
		log.Error(err, "failed to lookup release info")
		return nil, fmt.Errorf("failed to lookup release info: %w", err)
	}
	r.releaseInfoCache[imagePullSpec] = img
	return img, err
}

func (r *KubernetesServiceReconciler) reconcileEtcd(ctx context.Context, kubeSvc *hyperlitev1.KubernetesService) error {
	rootCASecret := pki.RootCASecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(rootCASecret), rootCASecret); err != nil {
		return fmt.Errorf("cannot get root CA secret: %w", err)
	}

	// Etcd client secret
	clientSecret := etcd.ClientSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(clientSecret), clientSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get client secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, clientSecret, func() error {
		ensureKSOwnerRef(kubeSvc, clientSecret)
		return etcd.ReconcileClientSecret(clientSecret, rootCASecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd client secret: %w", err)
	}

	// Etcd server secret
	serverSecret := etcd.ServerSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(clientSecret), clientSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get server secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, serverSecret, func() error {
		ensureKSOwnerRef(kubeSvc, serverSecret)
		return etcd.ReconcileServerSecret(serverSecret, rootCASecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd server secret: %w", err)
	}

	// Etcd peer secret
	peerSecret := etcd.PeerSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(clientSecret), clientSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get peer secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, peerSecret, func() error {
		ensureKSOwnerRef(kubeSvc, peerSecret)
		return etcd.ReconcilePeerSecret(peerSecret, rootCASecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd peer secret: %w", err)
	}

	// Etcd Operator ServiceAccount
	operatorServiceAccount := etcd.OperatorServiceAccount(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(operatorServiceAccount), operatorServiceAccount); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get etcd operator service account: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, operatorServiceAccount, func() error {
		ensureKSOwnerRef(kubeSvc, operatorServiceAccount)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd operator service account: %w", err)
	}

	// Etcd operator role
	operatorRole := etcd.OperatorRole(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(operatorRole), operatorRole); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get etcd operator role: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, operatorRole, func() error {
		ensureKSOwnerRef(kubeSvc, operatorRole)
		return etcd.ReconcileOperatorRole(operatorRole)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd operator role: %w", err)
	}

	// Etcd operator rolebinding
	operatorRoleBinding := etcd.OperatorRoleBinding(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(operatorRoleBinding), operatorRoleBinding); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get etcd operator role binding: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, operatorRoleBinding, func() error {
		ensureKSOwnerRef(kubeSvc, operatorRoleBinding)
		return etcd.ReconcileOperatorRoleBinding(operatorRoleBinding)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd operator role binding: %w", err)
	}

	// Etcd operator deployment
	operatorDeployment := etcd.OperatorDeployment(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(operatorDeployment), operatorDeployment); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get etcd operator deployment: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, operatorDeployment, func() error {
		ensureKSOwnerRef(kubeSvc, operatorDeployment)
		return etcd.ReconcileOperatorDeployment(operatorDeployment, etcdOperatorImage)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd operator deployment: %w", err)
	}

	// Etcd cluster
	etcdCluster := etcd.Cluster(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(etcdCluster), etcdCluster); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get etcd cluster: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, etcdCluster, func() error {
		ensureKSOwnerRef(kubeSvc, etcdCluster)
		return etcd.ReconcileCluster(etcdCluster, etcdClusterReplicas, etcdVersion)
	}); err != nil {
		return fmt.Errorf("failed to reconcile etcd cluster: %w", err)
	}

	return nil
}

func (r *KubernetesServiceReconciler) reconcileKubeAPIServer(ctx context.Context, kubeSvc *hyperlitev1.KubernetesService, imageInfo *releaseinfo.ReleaseImage) error {
	rootCASecret := pki.RootCASecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(rootCASecret), rootCASecret); err != nil {
		return fmt.Errorf("cannot get root CA secret: %w", err)
	}

	kubeAPIServerService := kas.Service(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerService), kubeAPIServerService); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server service: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerService, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerService)
		return kas.ReconcileService(kubeAPIServerService, kubeAPIServerPort, kubeAPIServerPort)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server service: %w", err)
	}

	kubeAPIServerCertSecret := kas.ServerCertSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerCertSecret), kubeAPIServerCertSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server cert secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerCertSecret, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerCertSecret)
		return kas.ReconcileServerCertSecret(kubeAPIServerCertSecret, rootCASecret, defaultServiceCIDR)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server cert secret: %w", err)
	}

	kubeAPIServerAggregatorCertSecret := kas.AggregatorCertSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerAggregatorCertSecret), kubeAPIServerAggregatorCertSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server aggreator cert secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerAggregatorCertSecret, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerAggregatorCertSecret)
		return kas.ReconcileAggregatorCertSecret(kubeAPIServerAggregatorCertSecret, rootCASecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server aggregator cert secret: %w", err)
	}

	serviceAccountSigningKeySecret := kas.ServiceAccountSigningKeySecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(serviceAccountSigningKeySecret), serviceAccountSigningKeySecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server service account key secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, serviceAccountSigningKeySecret, func() error {
		ensureKSOwnerRef(kubeSvc, serviceAccountSigningKeySecret)
		return kas.ReconcileServiceAccountSigningKeySecret(serviceAccountSigningKeySecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server service account key secret: %w", err)
	}

	serviceKubeconfigSecret := kas.ServiceKubeconfigSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(serviceKubeconfigSecret), serviceKubeconfigSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get service admin kubeconfig secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, serviceKubeconfigSecret, func() error {
		ensureKSOwnerRef(kubeSvc, serviceKubeconfigSecret)
		return kas.ReconcileServiceKubeconfigSecret(serviceKubeconfigSecret, rootCASecret, kubeAPIServerPort)
	}); err != nil {
		return fmt.Errorf("failed to reconcile service admin kubeconfig secret: %w", err)
	}

	localhostKubeconfigSecret := kas.LocalhostKubeconfigSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(localhostKubeconfigSecret), localhostKubeconfigSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get service localhost kubeconfig secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, localhostKubeconfigSecret, func() error {
		ensureKSOwnerRef(kubeSvc, localhostKubeconfigSecret)
		return kas.ReconcileLocalhostKubeconfigSecret(localhostKubeconfigSecret, rootCASecret, kubeAPIServerPort)
	}); err != nil {
		return fmt.Errorf("failed to reconcile localhost kubeconfig secret: %w", err)
	}

	kubeAPIServerAuditConfig := kas.AuditConfig(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerAuditConfig), kubeAPIServerAuditConfig); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server audit config: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerAuditConfig, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerAuditConfig)
		return kas.ReconcileAuditConfig(kubeAPIServerAuditConfig)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server audit config: %w", err)
	}

	kubeAPIServerConfig := kas.Config(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerConfig), kubeAPIServerConfig); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server config: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerConfig, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerConfig)
		return kas.ReconcileConfig(kubeAPIServerConfig, defaultServiceCIDR, kubeAPIServerPort)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server config: %w", err)
	}

	oauthMetadata := kas.OAuthMetadata(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(oauthMetadata), oauthMetadata); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get oauth metadata: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, oauthMetadata, func() error {
		ensureKSOwnerRef(kubeSvc, oauthMetadata)
		return kas.ReconcileOauthMetadata(oauthMetadata)
	}); err != nil {
		return fmt.Errorf("failed to reconcile oauth metadata: %w", err)
	}

	kubeAPIServerDeployment := kas.Deployment(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server deployment: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, kubeAPIServerDeployment, func() error {
		ensureKSOwnerRef(kubeSvc, kubeAPIServerDeployment)
		images := imageInfo.ComponentImages()
		return kas.ReconcileKubeAPIServerDeployment(
			kubeAPIServerDeployment,
			images["cluster-config-operator"],
			images["cli"],
			images["hyperkube"],
			kubeAPIServerPort,
			kubeAPIServerReplicas)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server service account key secret: %w", err)
	}

	return nil
}

func (r *KubernetesServiceReconciler) reconcileKubeControllerManager(ctx context.Context, kubeSvc *hyperlitev1.KubernetesService, imageInfo *releaseinfo.ReleaseImage) error {
	rootCASecret := pki.RootCASecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(rootCASecret), rootCASecret); err != nil {
		return fmt.Errorf("cannot get root CA secret: %w", err)
	}

	signerSecret := kcm.ClusterSignerSecret(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(signerSecret), signerSecret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get api server cert secret: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, signerSecret, func() error {
		ensureKSOwnerRef(kubeSvc, signerSecret)
		return kcm.ReconcileClusterSignerSecret(signerSecret, rootCASecret)
	}); err != nil {
		return fmt.Errorf("failed to reconcile api server cert secret: %w", err)
	}

	config := kcm.Config(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(config), config); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get controller manager config: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, config, func() error {
		ensureKSOwnerRef(kubeSvc, config)
		return kcm.ReconcileConfig(config)
	}); err != nil {
		return fmt.Errorf("failed to reconcile controller manager config: %w", err)
	}

	deployment := kcm.Deployment(kubeSvc.Namespace)
	if err := r.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get controller manager deployment: %w", err)
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r, deployment, func() error {
		images := imageInfo.ComponentImages()
		ensureKSOwnerRef(kubeSvc, deployment)
		return kcm.ReconcileDeployment(deployment, defaultPodCIDR, defaultServiceCIDR, images["hyperkube"], kubeControllerManagerReplicas)
	}); err != nil {
		return fmt.Errorf("failed to reconcile controller manager deployment: %w", err)
	}
	return nil
}

func ensureKSOwnerRef(kubeSvc *hyperlitev1.KubernetesService, object client.Object) {
	ownerRefs := object.GetOwnerReferences()
	newRefs := ensureOwnerRef(ownerRefs, metav1.OwnerReference{
		APIVersion:         hyperlitev1.GroupVersion.String(),
		Kind:               "KubernetesService",
		Name:               kubeSvc.GetName(),
		UID:                kubeSvc.UID,
		BlockOwnerDeletion: pointer.BoolPtr(true),
	})
	object.SetOwnerReferences(newRefs)
}

// EnsureOwnerRef makes sure the slice contains the OwnerReference.
func ensureOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) []metav1.OwnerReference {
	idx := indexOwnerRef(ownerReferences, ref)
	if idx == -1 {
		return append(ownerReferences, ref)
	}
	ownerReferences[idx] = ref
	return ownerReferences
}

// indexOwnerRef returns the index of the owner reference in the slice if found, or -1.
func indexOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
	for index, r := range ownerReferences {
		if referSameObject(r, ref) {
			return index
		}
	}
	return -1
}

// Returns true if a and b point to the same object.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}
