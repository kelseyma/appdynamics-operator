package clusteragent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"time"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_clusteragent")

const (
	AGENT_SECRET_NAME         string = "cluster-agent-secret"
	AGENT_CONFIG_NAME         string = "cluster-agent-config"
	AGENT_MON_CONFIG_NAME     string = "cluster-agent-mon"
	AGENT_LOG_CONFIG_NAME     string = "cluster-agent-log"
	AGENT_SSL_CONFIG_NAME     string = "appd-agent-ssl-config"
	AGENT_SSL_CRED_STORE_NAME string = "appd-agent-ssl-store"
	OLD_SPEC                  string = "cluster-agent-spec"
)

// Add creates a new Clusteragent Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusteragent{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusteragent-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Clusteragent
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clusteragent{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner Clusteragent
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clusteragent{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileClusteragent{}

// ReconcileClusteragent reconciles a Clusteragent object
type ReconcileClusteragent struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileClusteragent) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clusteragent...")

	// Fetch the Clusteragent instance
	clusterAgent := &appdynamicsv1alpha1.Clusteragent{}
	err := r.client.Get(context.TODO(), request.NamespacedName, clusterAgent)
	reqLogger.Info("Retrieved cluster agent.", "Image", clusterAgent.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Cluster Agent resource not found. The object must be deleted")
			r.cleanUp(nil)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Cluster Agent")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Cluster agent spec exists. Checking the corresponding deployment...")
	// Check if the agent already exists in the namespace
	existingDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: clusterAgent.Name, Namespace: clusterAgent.Namespace}, existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Removing the old instance of the configMap...")
		r.cleanUp(clusterAgent)
		reqLogger.Info("Cluster agent deployment does not exist. Creating...")
		reqLogger.Info("Checking the secret...")
		secret, esecret := r.ensureSecret(clusterAgent)
		if esecret != nil {
			reqLogger.Error(esecret, "Failed to create new Cluster Agent due to secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, esecret
		}
		reqLogger.Info("Checking the config map")
		econfig := r.ensureConfigMap(clusterAgent, secret, true)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Cluster Agent due to config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, econfig
		}
		// Define a new deployment for the cluster agent
		dep := r.newAgentDeployment(clusterAgent, secret)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(clusterAgent)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Cluster agent deployment exists. Checking for deltas with the current state...")
	// Ensure the deployment spec matches the new spec
	// Differentiate between breaking changes and benign updates
	// Check if secret has been recreated. if yes, restart pod
	secret, errsecret := r.ensureSecret(clusterAgent)
	if errsecret != nil {
		reqLogger.Error(errsecret, "Failed to get cluster agent config secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, errsecret
	}

	reqLogger.Info("Retrieving agent config map", "Deployment.Namespace", clusterAgent.Namespace)
	econfig := r.ensureConfigMap(clusterAgent, secret, false)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, econfig
	}

	breaking, updateDeployment := r.hasBreakingChanges(clusterAgent, existingDeployment, secret)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster agent pod...")
		errRestart := r.restartAgent(clusterAgent)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart cluster agent", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Breaking changes detected. Updating the the cluster agent deployment...")
		err = r.client.Update(context.TODO(), existingDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update ClusterAgent Deployment", "Deployment.Namespace", existingDeployment.Namespace, "Deployment.Name", existingDeployment.Name)
			return reconcile.Result{}, err
		}
	} else {

		reqLogger.Info("No breaking changes.", "clusterAgent.Namespace", clusterAgent.Namespace)

		statusErr := r.updateStatus(clusterAgent)
		if statusErr == nil {
			reqLogger.Info("Status updated. Exiting reconciliation loop.")
		} else {
			reqLogger.Info("Status not updated. Exiting reconciliation loop.")
		}
		return reconcile.Result{}, nil

	}

	reqLogger.Info("Exiting reconciliation loop.")
	return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *ReconcileClusteragent) updateStatus(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	clusterAgent.Status.LastUpdateTime = metav1.Now()

	if errInstance := r.client.Update(context.TODO(), clusterAgent); errInstance != nil {
		return fmt.Errorf("Unable to update clusteragent instance. %v", errInstance)
	}
	log.Info("ClusterAgent instance updated successfully", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)

	err := r.client.Status().Update(context.TODO(), clusterAgent)
	if err != nil {
		log.Error(err, "Failed to update cluster agent status", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
	} else {
		log.Info("ClusterAgent status updated successfully", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileClusteragent) hasBreakingChanges(clusterAgent *appdynamicsv1alpha1.Clusteragent, existingDeployment *appsv1.Deployment, secret *corev1.Secret) (bool, bool) {
	breakingChanges := false
	updateDeployment := false

	fmt.Println("Checking for breaking changes...")

	if existingDeployment.Annotations != nil {
		if oldJson, ok := existingDeployment.Annotations[OLD_SPEC]; ok && oldJson != "" {
			var oldSpec appdynamicsv1alpha1.Clusteragent
			errJson := json.Unmarshal([]byte(oldJson), &oldSpec)
			if errJson != nil {
				log.Error(errJson, "Unable to retrieve the old spec from annotations", "clusterAgent.Namespace", clusterAgent.Namespace, "clusterAgent.Name", clusterAgent.Name)
			}

			if oldSpec.Spec.ControllerUrl != clusterAgent.Spec.ControllerUrl || oldSpec.Spec.Account != clusterAgent.Spec.Account {
				breakingChanges = true
			}
		}
	}

	if clusterAgent.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != clusterAgent.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, clusterAgent.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = clusterAgent.Spec.Image
		return false, true
	}

	return breakingChanges, updateDeployment
}

func (r *ReconcileClusteragent) ensureSecret(clusterAgent *appdynamicsv1alpha1.Clusteragent) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: AGENT_SECRET_NAME}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the clusteragent will not start until at least the 'api-user' key of the secret has a valid value", AGENT_SECRET_NAME)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      AGENT_SECRET_NAME,
				Namespace: clusterAgent.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["controller-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", AGENT_SECRET_NAME)
			errLoad := r.client.Get(context.TODO(), key, secret)
			if errLoad != nil {
				fmt.Printf("Unable to reload secret. %v\n", errLoad)
				return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return secret, nil
}

func (r *ReconcileClusteragent) cleanUp(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	namespace := "appdynamics"
	if clusterAgent != nil {
		namespace = clusterAgent.Namespace
	}
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster-agent-config", Namespace: namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("The old instance of the configMap does not exist")
		return
	}

	if err == nil && cm != nil {
		err = r.client.Delete(context.TODO(), cm)
		if err != nil {
			log.Info("Unable to delete the old instance of the configMap", err)
		} else {
			log.Info("The old instance of the configMap deleted")
		}
	} else {
		log.Error(err, "Unable to retrieve the old instance of the configmap")
	}
}

func (r *ReconcileClusteragent) ensureConfigMap(clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret, create bool) error {
	setClusterAgentConfigDefaults(clusterAgent)

	err := r.ensureAgentMonConfig(clusterAgent)
	if err != nil {
		return err
	}

	err = r.ensureAgentConfig(clusterAgent)
	if err != nil {
		return err
	}

	err = r.ensureLogConfig(clusterAgent)

	return err

}
func validateControllerUrl(controllerUrl string) (error, string, uint16, string) {
	if strings.Contains(controllerUrl, "http") {
		arr := strings.Split(controllerUrl, ":")
		if len(arr) > 3 || len(arr) < 2 {
			return fmt.Errorf("Controller Url is invalid. Use this format: protocol://url:port"), "", 0, ""
		}
		protocol := arr[0]
		controllerDns := strings.TrimLeft(arr[1], "//")
		controllerPort := 0
		if len(arr) != 3 {
			if strings.Contains(protocol, "s") {
				controllerPort = 443
			} else {
				controllerPort = 80
			}
		} else {
			port, errPort := strconv.Atoi(arr[2])
			if errPort != nil {
				return fmt.Errorf("Controller port is invalid. %v", errPort), "", 0, ""
			}
			controllerPort = port
		}

		ssl := "false"
		if strings.Contains(protocol, "s") {
			ssl = "true"
		}
		return nil, controllerDns, uint16(controllerPort), ssl
	} else {
		return fmt.Errorf("Controller Url is invalid. Use this format: protocol://dns:port"), "", 0, ""
	}
}

func (r *ReconcileClusteragent) ensureAgentConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	errVal, controllerDns, port, sslEnabled := validateControllerUrl(clusterAgent.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}

	portVal := strconv.Itoa(int(port))

	cm.Name = AGENT_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] = clusterAgent.Spec.Account
	cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] = controllerDns
	cm.Data["APPDYNAMICS_CONTROLLER_PORT"] = portVal
	cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] = sslEnabled
	cm.Data["APPDYNAMICS_CLUSTER_NAME"] = clusterAgent.Spec.AppName

	cm.Data["APPDYNAMICS_CLUSTER_MONITORED_NAMESPACES"] = strings.Join(clusterAgent.Spec.NsToMonitor, ",")
	cm.Data["APPDYNAMICS_CLUSTER_EVENT_UPLOAD_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.EventUploadInterval)
	cm.Data["APPDYNAMICS_CLUSTER_CONTAINER_REGISTRATION_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.ContainerRegistrationInterval)
	cm.Data["APPDYNAMICS_CLUSTER_HTTP_CLIENT_TIMEOUT_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.HttpClientTimeout)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureAgentMonConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	yml := fmt.Sprintf(`metric-collection-interval-seconds: %d
cluster-metric-collection-interval-seconds: %d
metadata-collection-interval-seconds: %d
container-registration-batch-size: %d
container-registration-max-parallel-requests: %d
pod-registration-batch-size: %d 		
container-filter:
%s`, clusterAgent.Spec.MetricsSyncInterval, clusterAgent.Spec.ClusterMetricsSyncInterval, clusterAgent.Spec.MetadataSyncInterval,
		clusterAgent.Spec.ContainerBatchSize, clusterAgent.Spec.ContainerParallelRequestLimit, clusterAgent.Spec.PodBatchSize,
		createContainerFilterString(clusterAgent))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_MON_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	cm.Name = AGENT_MON_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent-monitoring.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureLogConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	yml := fmt.Sprintf(`log-level: %s
max-filesize-mb: %d
max-backups: %d
write-to-stdout: %s`, clusterAgent.Spec.LogLevel, clusterAgent.Spec.LogFileSizeMb, clusterAgent.Spec.LogFileBackups,
	strings.ToLower(clusterAgent.Spec.StdoutLogging))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent Log configMap. %v", err)
	}

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["logger-config.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureSSLConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {

	if clusterAgent.Spec.AgentSSLStoreName == "" {
		return nil
	}

	//verify that AGENT_SSL_CRED_STORE_NAME config map exists.
	//it will be copied into the respecive namespace for instrumentation

	existing := &corev1.ConfigMap{}
	errCheck := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CRED_STORE_NAME, Namespace: clusterAgent.Namespace}, existing)

	if errCheck != nil && errors.IsNotFound(errCheck) {
		return fmt.Errorf("Custom SSL store is requested, but the expected configMap %s with the trusted certificate store not found. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, clusterAgent.Namespace)
	} else if errCheck != nil {
		return fmt.Errorf("Unable to validate the expected configMap %s with the trusted certificate store. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, clusterAgent.Namespace)
	}

	return nil
}

func (r *ReconcileClusteragent) newAgentDeployment(clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret) *appsv1.Deployment {
	if clusterAgent.Spec.Image == "" {
		clusterAgent.Spec.Image = "appdynamics/cluster-agent:latest"
	}
	fmt.Printf("Building deployment spec for image %s\n", clusterAgent.Spec.Image)
	ls := labelsForClusteragent(clusterAgent)
	var replicas int32 = 1
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterAgent.Name,
			Namespace: clusterAgent.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "appdynamics-operator",
					Containers: []corev1.Container{{
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_CONFIG_NAME}}}},
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
										Key:                  "controller-key",
									},
								},
							},
							{
								Name: "APPDYNAMICS_AGENT_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Image:           clusterAgent.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-agent",
						Resources:       clusterAgent.Spec.Resources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "agent-mon-config",
							MountPath: "/opt/appdynamics/cluster-agent/config/agent-monitoring.yml",
							SubPath:   "agent-monitoring.yml",
						},
							{
								Name:      "agent-log",
								MountPath: "/opt/appdynamics/cluster-agent/config/logging/",
							}},
					}},
					Volumes: []corev1.Volume{{
						Name: "agent-mon-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_MON_CONFIG_NAME,
								},
							},
						},
					},
						{
							Name: "agent-log",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: AGENT_LOG_CONFIG_NAME,
									},
								},
							},
						}},
				},
			},
		},
	}

	//mount custom SSL cert if necessary
	if clusterAgent.Spec.CustomSSLSecret != "" {
		sslVol := corev1.Volume{
			Name: "agent-ssl-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: clusterAgent.Spec.CustomSSLSecret,
				},
			},
		}
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, sslVol)

		sslMount := corev1.VolumeMount{
			Name:      "agent-ssl-cert",
			MountPath: fmt.Sprintf("/opt/appdynamics/ssl/%s", clusterAgent.Spec.AgentSSLCert),
			SubPath:   clusterAgent.Spec.AgentSSLCert,
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, sslMount)
	}

	//save the new spec in annotations
	jsonObj, e := json.Marshal(clusterAgent)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "clusterAgent.Namespace", clusterAgent.Namespace, "clusterAgent.Name", clusterAgent.Name)
	} else {
		if dep.Annotations == nil {
			dep.Annotations = make(map[string]string)
		}
		dep.Annotations[OLD_SPEC] = string(jsonObj)
	}

	// Set Cluster Agent instance as the owner and controller
	controllerutil.SetControllerReference(clusterAgent, dep, r.scheme)
	return dep
}

func (r *ReconcileClusteragent) restartAgent(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForClusteragent(clusterAgent))
	listOps := &client.ListOptions{
		Namespace:     clusterAgent.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve cluster-agent pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = r.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete cluster-agent pod. %v", err)
	}
	return nil
}

func labelsForClusteragent(clusterAgent *appdynamicsv1alpha1.Clusteragent) map[string]string {
	return map[string]string{"name": "clusterAgent", "clusterAgent_cr": clusterAgent.Name}
}

func setClusterAgentConfigDefaults(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	// bootstrap-config defaults
	if clusterAgent.Spec.NsToMonitor == nil {
		clusterAgent.Spec.NsToMonitor = []string{"default"}
	}

	if clusterAgent.Spec.EventUploadInterval == 0 {
		clusterAgent.Spec.EventUploadInterval = 10
	}

	if clusterAgent.Spec.ContainerRegistrationInterval == 0 {
		clusterAgent.Spec.ContainerRegistrationInterval = 120
	}

	if clusterAgent.Spec.HttpClientTimeout == 0 {
		clusterAgent.Spec.HttpClientTimeout = 30
	}

	// agent-monitoring defaults
	if clusterAgent.Spec.MetricsSyncInterval == 0 {
		clusterAgent.Spec.MetricsSyncInterval = 30
	}

	if clusterAgent.Spec.ClusterMetricsSyncInterval == 0 {
		clusterAgent.Spec.ClusterMetricsSyncInterval = 60
	}

	if clusterAgent.Spec.MetadataSyncInterval == 0 {
		clusterAgent.Spec.MetadataSyncInterval = 60
	}

	if clusterAgent.Spec.ContainerBatchSize == 0 {
		clusterAgent.Spec.ContainerBatchSize = 25
	}

	if clusterAgent.Spec.ContainerParallelRequestLimit == 0 {
		clusterAgent.Spec.ContainerParallelRequestLimit = 3
	}

	if clusterAgent.Spec.PodBatchSize == 0 {
		clusterAgent.Spec.PodBatchSize = 30
	}

	if clusterAgent.Spec.ContainerFilter.WhitelistedNames == nil &&
			clusterAgent.Spec.ContainerFilter.BlacklistedNames == nil &&
			clusterAgent.Spec.ContainerFilter.BlacklistedLabels == nil {
		clusterAgent.Spec.ContainerFilter = appdynamicsv1alpha1.ClusteragentContainerFilter{
			BlacklistedLabels: map[string]string{"appdynamics.exclude": "true"},
		}
	}

	// logger defaults
	if clusterAgent.Spec.LogLevel == "" {
		clusterAgent.Spec.LogLevel = "INFO"
	}

	if clusterAgent.Spec.LogFileSizeMb == 0 {
		clusterAgent.Spec.LogFileSizeMb = 5
	}

	if clusterAgent.Spec.LogFileBackups == 0 {
		clusterAgent.Spec.LogFileBackups = 3
	}

	if clusterAgent.Spec.StdoutLogging == "" {
		clusterAgent.Spec.StdoutLogging = "true"
	}
}

func parseFilterField(fieldName string, fieldMap map[string][]string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(fmt.Sprintf("  %s: {", fieldName))
	for key, value := range fieldMap {
		stringBuilder.WriteString(fmt.Sprintf("%s: %s,", key, strings.Join(value, " ")))
	}
	filterString := strings.TrimRight(stringBuilder.String(), ",")
	return filterString + "}\n"
}

func createContainerFilterString(clusterAgent *appdynamicsv1alpha1.Clusteragent) string {
	var containerFilterString strings.Builder
	if clusterAgent.Spec.ContainerFilter.BlacklistedLabels != nil {
		var stringBuilder strings.Builder
		stringBuilder.WriteString("  blacklisted-label: {")
		for labelKey, labelValue := range clusterAgent.Spec.ContainerFilter.BlacklistedLabels {
			stringBuilder.WriteString(fmt.Sprintf("%s: %s,", labelKey, labelValue))
		}
		blacklistedLabels := strings.TrimRight(stringBuilder.String(), ",")
		containerFilterString.WriteString(blacklistedLabels + "}\n")
	}

	if clusterAgent.Spec.ContainerFilter.BlacklistedNames != nil {
		containerFilterString.WriteString(parseFilterField("blacklisted-names", clusterAgent.Spec.ContainerFilter.BlacklistedNames))
	}

	if clusterAgent.Spec.ContainerFilter.WhitelistedNames != nil {
		containerFilterString.WriteString(parseFilterField("whitelisted-names", clusterAgent.Spec.ContainerFilter.WhitelistedNames))
	}

	return containerFilterString.String()
}
