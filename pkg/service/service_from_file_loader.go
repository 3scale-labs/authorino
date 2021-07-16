package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	configv1beta1 "github.com/kuadrant/authorino/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const SERVICE_FILE_EXTENSION = ".json"

type ServiceFromFileLoader struct {
	client.Reader
	client.Writer
	BaseDir   string
	Namespace string
}

func NewServiceFromFileLoader(localDir string, namespace string) *ServiceFromFileLoader {
	dir := localDir
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}
	return &ServiceFromFileLoader{
		BaseDir:   dir,
		Namespace: namespace,
	}
}

// Get sets a `configv1beta1.Service` into `obj`, reading from file in the filesystem whose name is `key`
func (c *ServiceFromFileLoader) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	service := &configv1beta1.Service{}
	if err := c.readServiceFromFile(key.Name+SERVICE_FILE_EXTENSION, service); err != nil {
		return err
	} else {
		service.DeepCopyInto(obj.(*configv1beta1.Service))
		return nil
	}
}

// List sets a `configv1beta1.ServiceList{}` into `list`, reading from files in the filesystem
func (c *ServiceFromFileLoader) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	if files, err := c.serviceFileNames(); err != nil {
		return err
	} else {
		services := make([]configv1beta1.Service, 0)
		for _, file := range files {
			service := &configv1beta1.Service{}
			if err := c.readServiceFromFile(file, service); err != nil {
				return err
			} else {
				services = append(services, *service)
			}
		}
		list.(*configv1beta1.ServiceList).Items = services
		return nil
	}
}

func (c *ServiceFromFileLoader) Load(mgr manager.Manager, reconciler reconcile.Reconciler) error {
	ctx := context.Background()

	// wait for the cache to be ready
	mgrCacheSyncCtx, mgrCacheSyncCtxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer mgrCacheSyncCtxCancel()
	if synced := mgr.GetCache().WaitForCacheSync(mgrCacheSyncCtx.Done()); !synced {
		return fmt.Errorf("cache sync timeout")
	}

	// "reconcile" all
	var services = &configv1beta1.ServiceList{}

	if err := c.List(ctx, services); err != nil {
		return err
	} else {
		for _, service := range services.Items {
			serviceName := service.Name
			serviceNamespace := c.Namespace
			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: serviceNamespace,
					Name:      serviceName,
				},
			}
			if _, err = reconciler.Reconcile(request); err != nil {
				return err
			}
		}
		return nil
	}
}

// ServiceFromFileLoader implements client.Writer but it's a no-op

func (c *ServiceFromFileLoader) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

func (c *ServiceFromFileLoader) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

func (c *ServiceFromFileLoader) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

func (c *ServiceFromFileLoader) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (c *ServiceFromFileLoader) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (c *ServiceFromFileLoader) serviceFileNames() ([]string, error) {
	glob := fmt.Sprintf("%v*%v", c.BaseDir, SERVICE_FILE_EXTENSION)
	if files, err := filepath.Glob(glob); err != nil {
		return nil, err
	} else {
		fileNames := make([]string, 0)
		for _, fileName := range files {
			fileNames = append(fileNames, fileName[len(c.BaseDir):])
		}
		return fileNames, nil
	}
}

func (c *ServiceFromFileLoader) readServiceFromFile(fileName string, service *configv1beta1.Service) error {
	if fileContent, err := ioutil.ReadFile(c.BaseDir + fileName); err != nil {
		return err
	} else {
		var serviceSpec configv1beta1.ServiceSpec
		if err := json.Unmarshal(fileContent, &serviceSpec); err != nil {
			return err
		} else {
			serviceName := fileName[:len(fileName)-len(SERVICE_FILE_EXTENSION)] // remove extension of the file
			service.ObjectMeta = metav1.ObjectMeta{
				Namespace: c.Namespace,
				Name:      serviceName,
			}
			service.Spec = serviceSpec
			return nil
		}
	}
}
