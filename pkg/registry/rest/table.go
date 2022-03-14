/*
Copyright 2014 The Kubernetes Authors.

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

package rest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

// table
// 	表是一组 API 资源的表格表示。 服务器将对象转换为一组首选列，以便快速查看对象。
type defaultTableConvertor struct {
	// ["Group","Resource"]
	defaultQualifiedResource schema.GroupResource
}

// 创建一个默认convertor
// NewDefaultTableConvertor creates a default convertor; the provided resource is used for error messages
// if no resource info can be determined from the context passed to ConvertToTable.
func NewDefaultTableConvertor(defaultQualifiedResource schema.GroupResource) TableConvertor {
	return defaultTableConvertor{defaultQualifiedResource: defaultQualifiedResource}
}

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

func (c defaultTableConvertor) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table
	// 函数对象
	fn := func(obj runtime.Object) error {
		// 将对象转换为metav1.Object
		m, err := meta.Accessor(obj)
		if err != nil {
			resource := c.defaultQualifiedResource
			if info, ok := genericapirequest.RequestInfoFrom(ctx); ok {
				resource = schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
			}
			return errNotAcceptable{resource: resource}
		}
		// TableRow
		// 	1.Cells []interface{}:用于填写表格内容的方法
		// 	2.Conditions []TableRowCondition[可选]
		// conditions describe additional status of a row that are relevant for a human user. These conditions
		// apply to the row, not to the object, and will be specific to table output. The only defined
		// condition type is 'Completed', for a row that indicates a resource that has run to completion and
		// can be given less visual priority.
		// 	3.Object runtime.RawExtension[可选]
		// This field contains the requested additional information about each object based on the includeObject
		// policy when requesting the Table. If "None", this field is empty, if "Object" this will be the
		// default serialization of the object for the current API version, and if "Metadata" (the default) will
		// contain the object metadata. Check the returned kind and apiVersion of the object before parsing.
		// The media type of the object will always match the enclosing list - if this as a JSON table, these
		// will be JSON encoded objects.
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{m.GetName(), m.GetCreationTimestamp().Time.UTC().Format(time.RFC3339)},
			Object: runtime.RawExtension{Object: obj},
		})
		return nil
	}
	switch {
	case meta.IsListType(object):
		if err := meta.EachListItem(object, fn); err != nil {
			return nil, err
		}
	default:
		if err := fn(object); err != nil {
			return nil, err
		}
	}
	if m, err := meta.ListAccessor(object); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.SelfLink = m.GetSelfLink()
		table.Continue = m.GetContinue()
		table.RemainingItemCount = m.GetRemainingItemCount()
	} else {
		if m, err := meta.CommonAccessor(object); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
			table.SelfLink = m.GetSelfLink()
		}
	}
	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		// TableColumnDefinition
		//	1.Name
		//	2.Type(string):OpenAPI type(例:number, integer, string,array)
		//	3.Format(string):可选的OpenAPI type modifier
		// 		name:'name' format应用于primary identifier column(通常为资源的名字)
		//	4.description(string):人类可读
		//	5.Priority(int32):定义了该列相对于其他列的重要性(数字越小重要性越高)[在空间有限的情况下可能会省略的列应给予更高的优先级。]
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Created At", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		}
	}
	return &table, nil
}

// errNotAcceptable indicates the resource doesn't support Table conversion
type errNotAcceptable struct {
	resource schema.GroupResource
}

func (e errNotAcceptable) Error() string {
	return fmt.Sprintf("the resource %s does not support being converted to a Table", e.resource)
}

func (e errNotAcceptable) Status() metav1.Status {
	return metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusNotAcceptable,
		Reason:  metav1.StatusReason("NotAcceptable"),
		Message: e.Error(),
	}
}
