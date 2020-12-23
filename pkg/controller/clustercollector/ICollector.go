package clustercollector

import (
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IClusterController interface {
	Init(logger logr.Logger) (bool,error)
	Update(logger logr.Logger) (bool, error)
	Create(logger logr.Logger) error
	Get() metav1.Object
}
