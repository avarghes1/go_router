// go_router is a simple rest based router.
// The supported HTTP methods are GET, POST and DELETE.
// The url path has to be in the form `/version/resource/handler/param-name/param-value`.
//
// Json is the supported response type.
// It also supports the use of filters for pre and post dispatch process.
//
// @author: avarghese
package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	JSON = "application/json"
)

type (
	nodeMap   map[string]Node
	routeMap  map[string]nodeMap
	filterMap map[string]Filter
	Request   map[string]*RequestParam
	// Node is a controller function.
	// The function should have a pointer to all required request parameters.
	// Returns an interface and an error.
	// Example:
	//      type Test struct {
	//          Id int64
	//      }
	//      func GetUser(t *Test) (string, error) {
	//          fmt.Println(t)
	//          return "user", nil
	//      }
	//
	Node interface{}
	// Filters allow for pre and post dispatch work.
	// For example verifying api key.
	Filter interface {
		Name() string
		PreDispatch(*http.Request, Request) error
		PostDispatch(*http.Request, Request) error
	}
	RequestParam struct {
		Value interface{}
	}
)

var (
	routes  = make(routeMap)
	filters = make(filterMap)
)

// Get the controller associated with the incoming request.
func getNode(method string, path string) (Node, error) {
	if nodes, ok := routes[method]; ok {
		if v, ok := nodes[path]; ok {
			return v, nil
		}
	}
	return nil, errors.New("No Handler Found")
}

// Respond to a request where the controller is not found.
func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Resource Not Found.\n"))
}

// Respond to an unsupported request method.
func notSupported(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Request Method  is not supported.\n"))
}

// Respond to a request when something goes wrong.
func internalError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Server Error.\n"))
}

// Parse the incoming request url for parameters.
// supported url is in the form
// /version/resource/handler/{param-name}/{param}
func parseGet(r *http.Request, req Request) (string, error) {
	s := strings.Split(html.EscapeString(
		strings.TrimRight(r.URL.Path, "/")), "/")
	l := len(s)
	if l <= 3 || l%2 != 0 {
		return "", errors.New("Not Found")
	}
	for i := 4; i < l-1; i += 2 {
		t := RequestParam{Value: s[i+1]}
		req[s[i]] = &t
	}
	return strings.Join(s[0:4], "/"), nil
}

// Parse the request form for query parameters
// as well as post params.
func parseForm(r *http.Request, req Request) Request {
	for k, v := range r.Form {
		t := RequestParam{Value: v[0]}
		req[k] = &t
	}
	return req
}

// Parse the body for a json parameter. This is the
// accepted way of posting a request.
func parseBody(r *http.Request, req Request) (Request, error) {
	var i map[string]interface{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		// log the error and panic
		return req, err
	}
	err = json.Unmarshal(body, &i)
	if err != nil {
		// log the error and panic
		return req, err
	}
	for k, v := range i {
		req[k] = &RequestParam{Value: v}
	}
	return req, nil
}

// Run all registered filters predispatch function.
func preDispatch(r *http.Request, req Request) (err error) {
	for _, v := range filters {
		err = v.PreDispatch(r, req)
		if err != nil {
			return err
		}
	}
	return nil
}

// Run all registered filters postdispatch function.
func postDispatch(r *http.Request, req Request) (err error) {
	for _, v := range filters {
		err = v.PostDispatch(r, req)
		if err != nil {
			return err
		}
	}
	return nil
}

// function to get ensure first letter is caps
func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// Get an interger param
func (p *RequestParam) int() (int64, error) {
	switch p.Value.(type) {
	case string:
		return strconv.ParseInt(p.Value.(string), 10, 64)
	case int64:
		return p.Value.(int64), nil
	case float64:
		return int64(p.Value.(float64)), nil
	}
	return -1, errors.New("Not Found")
}

// Get a float param
func (p *RequestParam) float() (float64, error) {
	switch p.Value.(type) {
	case string:
		return strconv.ParseFloat(p.Value.(string), 64)
	case int64:
		return float64(p.Value.(int64)), nil
	case float64:
		return p.Value.(float64), nil
	}
	return -1, errors.New("Not Found")
}

// Get a boolean param
func (p *RequestParam) bool() (bool, error) {
	switch p.Value.(type) {
	case string:
		return strconv.ParseBool(p.Value.(string))
	case bool:
		return p.Value.(bool), nil
	}
	return false, errors.New("Not Found")
}

// This is responsible for setting up the input parameter of a handler
func setInputParam(i reflect.Value, req Request) (reflect.Value, error) {
	p := i.Type().In(0)
	t := reflect.New(p.Elem())
	for k, v := range req {
		k = upperFirst(k)
		sv, f := p.Elem().FieldByName(k)
		if !f {
			return t, errors.New("Not Found")
		}
		switch sv.Type.Kind() {
		case reflect.Int64:
			value, err := v.int()
			if err != nil {
				return t, err
			}
			t.Elem().FieldByName(k).SetInt(value)
		case reflect.Float64:
			value, err := v.float()
			if err != nil {
				return t, err
			}
			t.Elem().FieldByName(k).SetFloat(value)
		case reflect.Bool:
			value, err := v.bool()
			if err != nil {
				return t, err
			}
			t.Elem().FieldByName(k).SetBool(value)
		case reflect.String:
			t.Elem().FieldByName(k).SetString(v.Value.(string))
		default:
			return t, errors.New("Not Found")
		}
	}
	return t, nil
}

// Register a filter
//
//  Usage:
//
//      go_router.RegisterFilte("filter", test_filter)
//
func RegisterFilter(name string, f Filter) error {
	if _, ok := filters[name]; ok {
		return errors.New("Filter name is already registered")
	}
	filters[name] = f
	return nil
}

// Register a route.
// Parameters required are http method, url path and a controller.
//
//  Usage:
//
//      go_router.RegisterRoute(GET, "/v1/test/retrieve", test_controller.Retrieve)
//      go_router.RegisterRoute(POST, "/v1/test/save", test_controller.Save)
//
func RegisterRoute(method string, path string, n Node) error {
	if nodes, ok := routes[method]; ok {
		if _, ok := nodes[path]; ok {
			// log and return error
			return errors.New("Route path has already been registered")
		}
	}
	if _, ok := routes[method]; !ok {
		nodes := make(nodeMap)
		nodes[path] = n
		routes[method] = nodes
		return nil
	}
	nodes := routes[method]
	nodes[path] = n
	return nil
}

// Dispatch a Request.
// Only supports json responses.
//
//  Usage:
//
//      http.HandleFunc("/", router.Dispatch)
//      http.ListenAndServe(":8080", nil)
//
func Dispatch(w http.ResponseWriter, r *http.Request) {
	var routeKey string
	// make a map for request params
	req := make(Request)
	w.Header().Set("Content-Type", JSON)
	defer func() {
		if err := recover(); err != nil {
			// log the error using a logger.
			// log.Error(err)
			// print to terminal for now.
			fmt.Println(err)
			internalError(w, r)
		}
	}()
	err := r.ParseForm()
	if err != nil {
		// log the error and panic
		panic(err)
	}
	switch r.Method {
	case "GET", "DELETE":
		routeKey, err = parseGet(r, req)
		if err != nil {
			notFound(w, r)
			return
		}
	case "POST":
		routeKey = r.URL.Path
		req, err = parseBody(r, req)
		if err != nil {
			// log the error and panic
			panic(err)
		}
	default:
		notSupported(w, r)
		return
	}
	// get controller node from routes map.
	c, err := getNode(r.Method, routeKey)
	if err != nil {
		notFound(w, r)
		return
	}
	i := reflect.ValueOf(c)
	t, err := setInputParam(i, req)
	if err != nil {
		notFound(w, r)
		return
	}
	req = parseForm(r, req)
	err = preDispatch(r, req)
	if err != nil {
		// log the error and panic
		panic(err)
	}
	// invoke the controller.
	cont := i.Call([]reflect.Value{t})
	if !cont[1].IsNil() {
		err = cont[1].Interface().(error)
		if err != nil {
			// log the error and panic
			panic(err)
		}
	}
	err = postDispatch(r, req)
	if err != nil {
		// log the error and panic
		panic(err)
	}
	data, err := json.Marshal(cont[0].Interface())
	if err != nil {
		// log the error and panic
		panic(err)
	}
	fmt.Fprintf(w, "%s", string(data))
}
