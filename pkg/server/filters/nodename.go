package filters

import (
	"context"
	"net"
	"net/http"

	"github.com/loft-sh/vcluster/pkg/constants"
	"github.com/loft-sh/vcluster/pkg/controllers/resources/nodes/nodeservice"
	requestpkg "github.com/loft-sh/vcluster/pkg/util/request"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeName int

// nodeNameKey is the NodeName key for the context. It's of private type here. Because
// keys are interfaces and interfaces are equal when the type and the value is equal, this
// does not conflict with the keys defined in pkg/api.
const nodeNameKey nodeName = iota

func WithNodeName(h http.Handler, currentNamespace string, currentNamespaceClient client.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		nodeName, err := nodeNameFromHost(req.Context(), req.Host, currentNamespace, currentNamespaceClient)
		if err != nil {
			requestpkg.FailWithStatus(w, req, http.StatusInternalServerError, errors.Wrap(err, "find node name from host"))
			return
		} else if nodeName != "" {
			req = req.WithContext(context.WithValue(req.Context(), nodeNameKey, nodeName))
		}

		h.ServeHTTP(w, req)
	})
}

// NodeNameFrom returns a node name if there is any
func NodeNameFrom(ctx context.Context) (string, bool) {
	info, ok := ctx.Value(nodeNameKey).(string)
	return info, ok
}

func nodeNameFromHost(ctx context.Context, host, currentNamespace string, currentNamespaceClient client.Client) (string, error) {
	klog.Infof("Call net.ResolveUDPAddr(\"udp\", host) for %s host", host) //dev debug
	addr, err := net.ResolveUDPAddr("udp", host)
	if err == nil {
		clusterIP := addr.IP.String()
		klog.Infof("clusterIP is %s", clusterIP) //dev debug
		serviceList := &corev1.ServiceList{}
		err = currentNamespaceClient.List(ctx, serviceList, client.InNamespace(currentNamespace), client.MatchingFields{constants.IndexByClusterIP: clusterIP})
		if err != nil {
			return "", err
		}

		klog.Infof("len(serviceList.Items) = %d", len(serviceList.Items)) //dev debug
		// we found a service?
		if len(serviceList.Items) > 0 {
			serviceLabels := serviceList.Items[0].Labels
			if len(serviceLabels) > 0 && serviceLabels[nodeservice.ServiceNodeLabel] != "" {
				klog.Infof("service %s found, node %s", serviceList.Items[0].GetName(), serviceLabels[nodeservice.ServiceNodeLabel]) //dev debug
				return serviceLabels[nodeservice.ServiceNodeLabel], nil
			}
		}
	} else {
		klog.Errorf("Call net.ResolveUDPAddr(\"udp\", host) for %s host failed: %v", host, err) //dev debug
	}

	return "", nil
}
