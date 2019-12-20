/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"fmt"
	"github.com/keikoproj/iam-manager/internal/config"
	"github.com/keikoproj/iam-manager/internal/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"strings"
)

const (
	version = "2012-10-17"
)

// log is for logging in this package.
var iamrolelog = logf.Log.WithName("iamrole-resource")

var wClient *k8s.Client
var props *config.Properties

func NewWClient() {
	fmt.Println("calling k8 client")
	k8sClient, err := k8s.NewK8sClient()
	if err != nil {
		panic(err)
	}
	wClient = k8sClient

	// call loadProperties with config map result
	props = config.LoadProperties(context.Background(), k8sClient, "test-test-test-usw2-dev-dev", "iam-manager-iamroles-v1alpha1-configmap")

}

func (r *Iamrole) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-iammanager-keikoproj-io-v1alpha1-iamrole,mutating=true,failurePolicy=fail,groups=iammanager.keikoproj.io,resources=iamroles,verbs=create;update,versions=v1alpha1,name=miamrole.kb.io

var _ webhook.Defaulter = &Iamrole{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Iamrole) Default() {
	iamrolelog.Info("default", "name", r.Name)

	//Set the default value for Version
	if r.Spec.PolicyDocument.Version == "" {
		r.Spec.PolicyDocument.Version = version
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-iammanager-keikoproj-io-v1alpha1-iamrole,mutating=false,failurePolicy=fail,groups=iammanager.keikoproj.io,resources=iamroles,versions=v1alpha1,name=viamrole.kb.io

var _ webhook.Validator = &Iamrole{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Iamrole) ValidateCreate() error {
	iamrolelog.Info("validate create", "name", r.Name)

	return r.validateIAMPolicy(false)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Iamrole) ValidateUpdate(old runtime.Object) error {
	iamrolelog.Info("validate update", "name", r.Name)

	return r.validateIAMPolicy(true)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Iamrole) ValidateDelete() error {
	iamrolelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Iamrole) validateIAMPolicy(isItUpdate bool) error {
	var allErrs field.ErrorList
	if err := r.validateCustomResourceName(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIAMPolicyAction(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIAMPolicyResource(); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.validateNumberOfRoles(isItUpdate); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "iammanager.keikoproj.io", Kind: "Iamrole"},
		r.Name, allErrs)
}

func (r *Iamrole) validateIAMPolicyAction() *field.Error {
	//Check the incoming policy actions
	for _, statement := range r.Spec.PolicyDocument.Statement {
		for _, action := range statement.Action {
			isAllowed := false
			for _, prefix := range props.AllowedPolicyAction {

				if strings.HasPrefix(action, prefix) {
					isAllowed = true
					break
				}

			}
			//This line shouldn't be executed unless if there is restricted action or end of the loop
			if !isAllowed {
				return field.Forbidden(field.NewPath("spec").Child("PolicyDocument").Child("Action"), fmt.Sprintf("restricted action %s included in the request", action))
			}
			//This is special case-- May be only for Intuit
			if strings.HasPrefix(action, "s3:") {
				for _, resource := range statement.Resource {
					for _, res := range props.RestrictedS3Resources {
						isAllowed := false
						if resource != res {
							isAllowed = true
							break
						}

						//This line shouldn't be executed unless if there is restricted action or end of the loop
						if !isAllowed {
							return field.Forbidden(field.NewPath("spec").Child("PolicyDocument").Child("Resource"), fmt.Sprintf("restricted resource %s included in the request", resource))
						}
					}
				}
			}
		}
	}
	return nil
}

func (r *Iamrole) validateIAMPolicyResource() *field.Error {
	//Check the incoming policy resource
	for _, statement := range r.Spec.PolicyDocument.Statement {
		for _, resource := range statement.Resource {
			isAllowed := true
			for _, res := range props.RestrictedPolicyResources {

				if strings.Contains(resource, res) {
					isAllowed = false
					break
				}
			}
			//This line shouldn't be executed unless if there is restricted action or end of the loop
			if !isAllowed {
				return field.Forbidden(field.NewPath("spec").Child("PolicyDocument").Child("Resource"), fmt.Sprintf("restricted resource %s included in the request", resource))
			}
		}
	}
	return nil
}

/*
Validating the length of a string field can be done declaratively by
the validation schema.
But the `ObjectMeta.Name` field is defined in a shared package under
the apimachinery repo, so we can't declaratively validate it using
the validation schema.
*/

func (r *Iamrole) validateCustomResourceName() *field.Error {
	if len(r.ObjectMeta.Name) > validationutils.DNS1035LabelMaxLength-11 {
		// The job name length is 63 character like all Kubernetes objects
		// (which must fit in a DNS subdomain). The cronjob controller appends
		// a 11-character suffix to the cronjob (`-$TIMESTAMP`) when creating
		// a job. The job name length limit is 63 characters. Therefore cronjob
		// names must have length <= 63-11=52. If we don't validate this here,
		// then job creation will fail later.
		return field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "must be no more than 52 characters")
	}
	return nil
}

//Lets do a cheesy way to talk to API server

func (r *Iamrole) validateNumberOfRoles(isItUpdate bool) *field.Error {
	count, err := wClient.IamrolesCount(context.Background(), r.ObjectMeta.Namespace)
	if err != nil {
		panic(err)
	}

	if isItUpdate {
		if count > 1 {
			return field.Invalid(field.NewPath("metadata").Child("namespace"), r.ObjectMeta.Namespace, "only 1 role is allowed per namespace")
		}
	} else {
		if count >= 1 {
			return field.Invalid(field.NewPath("metadata").Child("namespace"), r.ObjectMeta.Namespace, "only 1 role is allowed per namespace")
		}
	}
	//While doing update it should be fine to have

	return nil
}