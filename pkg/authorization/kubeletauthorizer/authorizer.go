package kubeletauthorizer

import (
	"context"

	"github.com/loft-sh/vcluster/pkg/server/filters"
	"github.com/loft-sh/vcluster/pkg/util/clienthelper"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

type PathVerb struct {
	Path string
	Verb string
}

func New(localManager ctrl.Manager, virtualManager ctrl.Manager) authorizer.Authorizer {
	return &kubeletAuthorizer{
		localManager:   localManager,
		virtualManager: virtualManager,
	}
}

type kubeletAuthorizer struct {
	localManager   ctrl.Manager
	virtualManager ctrl.Manager
}

func (l *kubeletAuthorizer) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) { // get node name
	nodeName, ok := filters.NodeNameFrom(ctx)
	klog.Infof("Checking %s users authorization for %s. Nodename: %s", a.GetUser().GetName(), a.GetPath(), nodeName) //dev debug
	if !ok {
		return authorizer.DecisionNoOpinion, "", nil
	} else if a.IsResourceRequest() {
		return authorizer.DecisionDeny, "forbidden", nil
	}

	// get cluster client
	client := l.virtualManager.GetClient()

	// check if request is allowed in the target cluster
	accessReview := &authv1.SubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: authv1.SubjectAccessReviewSpec{
			User:   a.GetUser().GetName(),
			UID:    a.GetUser().GetUID(),
			Groups: a.GetUser().GetGroups(),
			Extra:  clienthelper.ConvertExtra(a.GetUser().GetExtra()),
		},
	}

	// check what kind of request it is
	if filters.IsKubeletStats(a.GetPath()) {
		accessReview.Spec.ResourceAttributes = &authv1.ResourceAttributes{
			Verb:        "get",
			Group:       corev1.SchemeGroupVersion.Group,
			Version:     corev1.SchemeGroupVersion.Version,
			Resource:    "nodes",
			Subresource: "stats",
			Name:        nodeName,
		}
	} else if filters.IsKubeletMetrics(a.GetPath()) {
		accessReview.Spec.ResourceAttributes = &authv1.ResourceAttributes{
			Verb:        "get",
			Group:       corev1.SchemeGroupVersion.Group,
			Version:     corev1.SchemeGroupVersion.Version,
			Resource:    "nodes",
			Subresource: "metrics",
			Name:        nodeName,
		}
	} else {
		accessReview.Spec.NonResourceAttributes = &authv1.NonResourceAttributes{
			Path: a.GetPath(),
			Verb: a.GetVerb(),
		}
	}

	err = client.Create(ctx, accessReview)
	if err != nil {
		klog.Errorf("error creating access review: %v", err) //dev debug
		return authorizer.DecisionDeny, "", err
	} else if accessReview.Status.Allowed && !accessReview.Status.Denied {
		return authorizer.DecisionAllow, "", nil
	}

	klog.Info("return authorizer.DecisionDeny, accessReview.Status.Reason, nil") //dev debug
	return authorizer.DecisionDeny, accessReview.Status.Reason, nil
}
