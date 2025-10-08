package v0_2

import (
	"context"
	"sort"
	"strings"

	manager_http "github.com/foxxorcat/wazero-wasip2/manager/http"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"
)

// fieldsImpl 封装了 fields 资源的所有操作。
type fieldsImpl struct {
	fm *manager_http.FieldsManager
}

func newFieldsImpl(fm *manager_http.FieldsManager) *fieldsImpl {
	return &fieldsImpl{fm: fm}
}

// Constructor 实现了 [constructor]fields。
func (i *fieldsImpl) Constructor() Fields {
	return i.fm.Add(make(manager_http.Fields))
}

// FromListConstructor 实现了 [constructor]fields.from-list。
func (i *fieldsImpl) FromList(_ context.Context, entries []witgo.Tuple[FieldKey, FieldValue]) witgo.Result[Fields, HeaderError] {
	fields := make(manager_http.Fields)
	for _, entry := range entries {
		// HTTP 头部名称是不区分大小写的，我们统一转为小写。
		key := strings.ToLower(entry.F0)
		fields[key] = append(fields[key], string(entry.F1))
	}
	handle := i.fm.Add(fields)
	return witgo.Ok[Fields, HeaderError](handle)
}

// Drop 是 fields 资源的析构函数。
func (i *fieldsImpl) Drop(_ context.Context, handle Fields) {
	i.fm.Remove(handle)
}

// Get 实现了 [method]fields.get。
func (i *fieldsImpl) Get(_ context.Context, this Fields, name FieldKey) []FieldValue {
	f, ok := i.fm.Get(this)
	if !ok {
		return nil
	}
	values := f[strings.ToLower(name)]
	ret := make([]FieldValue, len(values))
	for j, v := range values {
		ret[j] = FieldValue(v)
	}
	return ret
}

// Get 实现了 [method]fields.has。
func (i *fieldsImpl) Has(_ context.Context, this Fields, name FieldKey) bool {
	_, ok := i.fm.Get(this)
	if !ok {
		return false
	}
	return true
}

// Set 实现了 [method]fields.set。
func (i *fieldsImpl) Set(_ context.Context, this Fields, name FieldKey, value []FieldValue) witgo.Result[witgo.Unit, HeaderError] {
	f, ok := i.fm.Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, HeaderError](HeaderError{Immutable: &witgo.Unit{}})
	}
	values := make([]string, len(value))
	for j, v := range value {
		values[j] = string(v)
	}
	f[strings.ToLower(name)] = values
	return witgo.Ok[witgo.Unit, HeaderError](witgo.Unit{})
}

// Delete 实现了 [method]fields.delete。
func (i *fieldsImpl) Delete(_ context.Context, this Fields, name FieldKey) witgo.Result[witgo.Unit, HeaderError] {
	f, ok := i.fm.Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, HeaderError](HeaderError{Immutable: &witgo.Unit{}})
	}
	delete(f, strings.ToLower(name))
	return witgo.Ok[witgo.Unit, HeaderError](witgo.Unit{})
}

// Append 实现了 [method]fields.append。
func (i *fieldsImpl) Append(_ context.Context, this Fields, name FieldKey, value FieldValue) witgo.Result[witgo.Unit, HeaderError] {
	f, ok := i.fm.Get(this)
	if !ok {
		return witgo.Err[witgo.Unit, HeaderError](HeaderError{Immutable: &witgo.Unit{}})
	}
	key := strings.ToLower(name)
	f[key] = append(f[key], string(value))
	return witgo.Ok[witgo.Unit, HeaderError](witgo.Unit{})
}

// Entries 实现了 [method]fields.entries。
func (i *fieldsImpl) Entries(_ context.Context, this Fields) []witgo.Tuple[FieldKey, FieldValue] {
	f, ok := i.fm.Get(this)
	if !ok {
		return nil
	}
	// 为了确保稳定的返回顺序，我们对 key 进行排序。
	keys := make([]FieldKey, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var entries []witgo.Tuple[FieldKey, FieldValue]
	for _, k := range keys {
		for _, v := range f[k] {
			entries = append(entries, witgo.Tuple[FieldKey, FieldValue]{F0: k, F1: FieldValue(v)})
		}
	}
	return entries
}

// Clone 实现了 [method]fields.clone。
func (i *fieldsImpl) Clone(_ context.Context, this Fields) Fields {
	f, ok := i.fm.Get(this)
	if !ok {
		// 如果源句柄无效，创建一个空的 fields 并返回。
		return i.Constructor()
	}

	// 创建一个新的 map 并深拷贝所有键值对。
	newFields := make(manager_http.Fields, len(f))
	for k, v := range f {
		newValues := make([]string, len(v))
		copy(newValues, v)
		newFields[k] = newValues
	}
	return i.fm.Add(newFields)
}
