package node_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	testingk8s "k8s.io/client-go/testing"

	"github.com/aws/eks-hybrid/internal/node"
)

func TestGetPodsOnNode(t *testing.T) {
	tests := []struct {
		name            string
		setupFakeClient func() kubernetes.Interface
		podCount        int
		expectedError   string
	}{
		{
			name: "get pods success",
			setupFakeClient: func() kubernetes.Interface {
				client := fake.NewSimpleClientset()
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				}
				_, _ = client.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
				return client
			},
			podCount: 1,
		},
		{
			name: "no pods on node",
			setupFakeClient: func() kubernetes.Interface {
				return fake.NewSimpleClientset()
			},
			podCount: 0,
		},
		{
			name: "consecutive failures",
			setupFakeClient: func() kubernetes.Interface {
				client := fake.NewSimpleClientset()
				client.PrependReactor("list", "pods", func(action testingk8s.Action) (handled bool, ret runtime.Object, err error) {
					return true, &corev1.PodList{}, errors.New("")
				})
				return client
			},
			expectedError: "listing all pods running on the node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			client := tt.setupFakeClient()
			ctx := context.Background()
			// Make sure tests that are supposed to fail fail fast
			ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
			defer cancel()

			pods, err := node.GetPodsOnNode(ctx, "test-node", client)

			if tt.expectedError != "" {
				g.Expect(err).ToNot(BeNil())
				g.Expect(err).To(MatchError(ContainSubstring(tt.expectedError)))
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(len(pods)).To(Equal(tt.podCount))
			}
		})
	}
}
