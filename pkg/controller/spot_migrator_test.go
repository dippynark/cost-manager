package controller

import (
	"context"
	"testing"
	"time"

	cloudproviderfake "github.com/hsbc/cost-manager/pkg/cloudprovider/fake"
	"github.com/stretchr/testify/require"
	"gopkg.in/robfig/cron.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestSpotMigratorNodeCreatedFalseOnNoChange(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("1"),
			},
		},
	}
	require.False(t, nodeCreated(nodes, nodes))
}

func TestSpotMigratorNodeCreatedFalseOnNodeRemoved(t *testing.T) {
	beforeNodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("1"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("2"),
			},
		},
	}
	afterNodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("1"),
			},
		},
	}
	require.False(t, nodeCreated(beforeNodes, afterNodes))
}

func TestSpotMigratorNodeCreatedTrueOnNodeCreate(t *testing.T) {
	beforeNodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("1"),
			},
		},
	}
	afterNodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("1"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("2"),
			},
		},
	}
	require.True(t, nodeCreated(beforeNodes, afterNodes))
}

func TestSpotMigratorChooseNodeToDrainErrorOnEmptyList(t *testing.T) {
	nodes := []*corev1.Node{}
	_, err := selectNodeForDeletion(context.TODO(), nodes)
	require.NotNil(t, err)
}

func TestSpotMigratorChooseNodeToDrainPreferOldest(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "secondoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://secondoldest",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://oldest",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "thirdoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(3 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://thirdoldest",
			},
		},
	}
	node, err := selectNodeForDeletion(context.TODO(), nodes)
	require.Nil(t, err)
	require.Equal(t, "oldest", node.Name)
}

func TestSpotMigratorChooseNodeToDrainPreferNodesMarkedPreferNoScheduleByClusterAutoscaler(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "secondoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://secondoldest",
				Taints: []corev1.Taint{
					{
						Key:    "DeletionCandidateOfClusterAutoscaler",
						Effect: corev1.TaintEffectPreferNoSchedule,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://oldest",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "thirdoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(3 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://thirdoldest",
			},
		},
	}
	node, err := selectNodeForDeletion(context.TODO(), nodes)
	require.Nil(t, err)
	require.Equal(t, "secondoldest", node.Name)
}

func TestSpotMigratorChooseNodeToDrainPreferNodesMarkedNoScheduleByClusterAutoscaler(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "secondoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://secondoldest",
				Taints: []corev1.Taint{
					{
						Key:    "DeletionCandidateOfClusterAutoscaler",
						Effect: corev1.TaintEffectPreferNoSchedule,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://oldest",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "thirdoldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				ProviderID: "gce://thirdoldest",
				Taints: []corev1.Taint{
					{
						Key:    "ToBeDeletedByClusterAutoscaler",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	}
	node, err := selectNodeForDeletion(context.TODO(), nodes)
	require.Nil(t, err)
	require.Equal(t, "thirdoldest", node.Name)
}

func TestSpotMigratorChooseNodeToDrainPreferUnschedulable(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(3 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: true,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
		},
	}
	node, err := selectNodeForDeletion(context.TODO(), nodes)
	require.Nil(t, err)
	require.True(t, node.Spec.Unschedulable)
}

func TestSpotMigratorChooseNodeToDrainPreferSelectedForDeletion(t *testing.T) {
	nodes := []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(3 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: true,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(2 * time.Hour),
				},
				Labels: map[string]string{
					"cost-manager.io/selected-for-deletion": "true",
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "oldest",
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(1 * time.Hour),
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
		},
	}
	node, err := selectNodeForDeletion(context.TODO(), nodes)
	require.Nil(t, err)
	require.True(t, isSelectedForDeletion(node))
}

// TestCronSpecHasFixedActivationTimes ensures that the cron spec does not return activation times
// that are a fixed amount of time ahead of the given time; otherwise, spot migration will never run
// if cost-manager is restarting more regularly than the activation interval. For example, using
// `@every 1h` for the cron spec would fail this test
func TestSpotMigratorCronSpecHasFixedActivationTimes(t *testing.T) {
	cronSchedule, err := cron.Parse(cronSpec)
	require.Nil(t, err)

	testTime := time.Date(00, 00, 00, 00, 00, 00, 00, time.UTC)
	require.Equal(t, cronSchedule.Next(testTime), cronSchedule.Next(testTime.Add(time.Second)))
}

func TestSpotMigratorPrometheusMetricRegistration(t *testing.T) {
	// Create cancelled context so that spot-migrator returns after starting
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Start spot-migrator
	err := (&SpotMigrator{
		Clientset: fake.NewSimpleClientset(),
	}).Start(ctx)
	require.Nil(t, err)

	// Make sure Prometheus metric has been registered
	metricFamilies, err := metrics.Registry.Gather()
	require.Nil(t, err)
	spotMigratorDrainSuccessMetricFound := false
	for _, metricFamily := range metricFamilies {
		// This metric name should match with the corresponding PrometheusRule alert
		if metricFamily.Name != nil && *metricFamily.Name == "cost_manager_spot_migrator_operation_success_total" {
			spotMigratorDrainSuccessMetricFound = true
		}
	}
	require.True(t, spotMigratorDrainSuccessMetricFound)
}

func TestAnnotateNode(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()
	sm := &SpotMigrator{
		Clientset: clientset,
	}
	node, err := clientset.CoreV1().Nodes().Create(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}, metav1.CreateOptions{})
	require.Nil(t, err)
	err = sm.addSelectedForDeletionLabel(ctx, node.Name)
	require.Nil(t, err)
	node, err = clientset.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	require.Nil(t, err)
	require.True(t, isSelectedForDeletion(node))
}

func TestIsSelectedForDeletion(t *testing.T) {
	tests := map[string]struct {
		node                  *corev1.Node
		isSelectedForDeletion bool
	}{
		"hasSelectedForDeletionLabelTrue": {
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cost-manager.io/selected-for-deletion": "true",
					},
				},
			},
			isSelectedForDeletion: true,
		},
		"hasSelectedForDeletionLabelFalse": {
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"cost-manager.io/selected-for-deletion": "false",
					},
				},
			},
			isSelectedForDeletion: false,
		},
		"hasAnotherLabel": {
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			isSelectedForDeletion: false,
		},
		"hasNoLabels": {
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				}},
			isSelectedForDeletion: false,
		},
		"missingLabels": {
			node:                  &corev1.Node{},
			isSelectedForDeletion: false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isSelectedForDeletion := isSelectedForDeletion(test.node)
			require.Equal(t, test.isSelectedForDeletion, isSelectedForDeletion)
		})
	}
}

func TestExcludeNodeFromExternalLoadBalancing(t *testing.T) {
	ctx := context.Background()
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	sm := &SpotMigrator{
		Clientset: fake.NewSimpleClientset(node),
	}

	err := sm.excludeNodeFromExternalLoadBalancing(ctx, node)
	require.Nil(t, err)

	node, err = sm.Clientset.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	require.Nil(t, err)

	// Verify that the cluster autoscaler taint was added
	hasToBeDeletedTaint := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == "ToBeDeletedByClusterAutoscaler" && taint.Effect == "NoSchedule" {
			hasToBeDeletedTaint = true
			break
		}
	}
	require.True(t, hasToBeDeletedTaint)

	// Verify that the exclusion label was added
	_, ok := node.Labels["node.kubernetes.io/exclude-from-external-load-balancers"]
	require.True(t, ok)
}

func TestListOnDemandNodes(t *testing.T) {
	tests := map[string]struct {
		nodes         []*corev1.Node
		onDemandNodes []*corev1.Node
	}{
		"noNodes": {
			nodes:         []*corev1.Node{},
			onDemandNodes: []*corev1.Node{},
		},
		"oneSpotNode": {
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							cloudproviderfake.SpotInstanceLabelKey: cloudproviderfake.SpotInstanceLabelValue,
						},
					},
				},
			},
			onDemandNodes: []*corev1.Node{},
		},
		"oneOnDemandNode": {
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			onDemandNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
		},
		"multipleNodes": {
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Labels: map[string]string{
							cloudproviderfake.SpotInstanceLabelKey: cloudproviderfake.SpotInstanceLabelValue,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
			onDemandNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			var objects []runtime.Object
			for _, node := range test.nodes {
				objects = append(objects, node)
			}
			sm := &SpotMigrator{
				Clientset:     fake.NewSimpleClientset(objects...),
				CloudProvider: &cloudproviderfake.CloudProvider{},
			}

			onDemandNodes, err := sm.listOnDemandNodes(ctx)
			require.Nil(t, err)
			require.Equal(t, test.onDemandNodes, onDemandNodes)
		})
	}
}
