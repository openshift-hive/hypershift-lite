package api

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	hyperlitev1 "github.com/openshift-hive/hypershiftlite/pkg/api/v1alpha1"
	etcdv1 "github.com/openshift-hive/hypershiftlite/thirdparty/etcd/v1beta2"
)

var (
	Scheme = runtime.NewScheme()
)

func init() {
	clientgoscheme.AddToScheme(Scheme)
	hyperlitev1.AddToScheme(Scheme)
	etcdv1.AddToScheme(Scheme)
}
