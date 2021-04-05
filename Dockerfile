FROM registry.ci.openshift.org/openshift/release:golang-1.16 as builder

WORKDIR /hypershift-lite

COPY . .

RUN make

FROM quay.io/openshift/origin-base:4.7
COPY --from=builder /hypershift-lite/bin/hypershift-lite /usr/bin/hypershift-lite

ENTRYPOINT /usr/bin/hypershift
