// @author: avarghese
// Simple rest router

package go_router

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
)

const (
	INT    = "{Int}"
	FLOAT  = "{Float}"
	BOOL   = "{Bool}"
	STRING = "{String}"
	JSON   = "application/json"
)

type (
	Request   map[string]Param
	NodeMap   map[string]Node
	RouteMap  map[string]NodeMap
	FilterMap map[string]Filter
	// Node is a controller function.
	// It accepts a request map.
	// Returns an interface and an error.
	Node interface{}
	// Filters allow for pre and post dispatch work.
	// For example verifying api key.
	Filter interface {
		Name() string
		PreDispatch(*http.Request, Request) error
		PostDispatch(*http.Request, Request) error
	}
	Param interface {
		Int() int64
		Float() float64
		Bool() bool
		String() string
	}
	RequestParam struct {
		Type  string
		Value interface{}
	}
)

var (
	routes  = make(RouteMap)
	filters = make(FilterMap)
)

// Function returns a request parameter. Accepted types are int64,
// float64, bool and string.
func toParam(i interface{}) RequestParam {
	if v, err := strconv.ParseInt(i.(string), 10, 64); err == nil {
		return RequestParam{Value: v, Type: INT}
	}
	if v, err := strconv.ParseFloat(i.(string), 64); err == nil {
		return RequestParam{Value: v, Type: FLOAT}
	}
	if v, err := strconv.ParseBool(i.(string)); err == nil {
		return RequestParam{Value: v, Type: BOOL}
	}
	return RequestParam{Value: i.(string), Type: STRING}
}

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
func parseGet(r *http.Request, req Request) string {
	s := strings.Split(html.EscapeString(
		strings.TrimRight(r.URL.Path, "/")), "/")
	l := len(s)
	for i := 4; i < l; i += 2 {
		t := toParam(s[i+1])
		req[s[i]] = &t
		s[i+1] = t.Type
	}
	return strings.Join(s, "/")
}

// Parse the request form for query parameters
// as well as post params.
func parseForm(r *http.Request, req Request) Request {
	for k, v := range r.Form {
		t := toParam(v[0])
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

// Get an interger param
func (p *RequestParam) Int() int64 {
	return p.Value.(int64)
}

// Get a float param
func (p *RequestParam) Float() float64 {
	return p.Value.(float64)
}

// Get a boolean param
func (p *RequestParam) Bool() bool {
	return p.Value.(bool)
}

// Get a string param
func (p *RequestParam) String() string {
	return p.Value.(string)
}

// Register a filter.
func RegisterFilter(name string, f Filter) error {
	if _, ok := filters[name]; ok {
		return errors.New("Filter name is already registered")
	}
	filters[name] = f
	return nil
}

// Register a route.
// Parameters required are http method, url path and a controller.
func RegisterRoute(method string, path string, n Node) error {
	if nodes, ok := routes[method]; ok {
		if _, ok := nodes[path]; ok {
			// log and return error
			return errors.New("Route path has already been registered")
		}
	}
	if _, ok := routes[method]; !ok {
		nodes := make(NodeMap)
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
func Dispatch(w http.ResponseWriter, r *http.Request) {
	var routeKey string
	// make a map for request params
	req := make(Request)
	w.Header().Set("Content-Type", JSON)
	defer func() {
		if err := recover(); err != nil {
			// log the error using a logger.
			// log.Error(err)
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
		routeKey = parseGet(r, req)
		req = parseForm(r, req)
	case "POST":
		routeKey = r.URL.Path
		req, err = parseBody(r, req)
		if err != nil {
			// log the error and panic
			panic(err)
		}
		req = parseForm(r, req)
	default:
		notSupported(w, r)
	}
	err = preDispatch(r, req)
	if err != nil {
		// log the error and panic
		panic(err)
	}
	c, err := getNode(r.Method, routeKey)
	if err != nil {
		notFound(w, r)
		return
	}
	i := reflect.ValueOf(c).Call([]reflect.Value{reflect.ValueOf(req)})
	if !i[1].IsNil() {
		err = i[1].Interface().(error)
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
	data, err := json.Marshal(i[0].Interface())
	if err != nil {
		// log the error and panic
		panic(err)
	}
	fmt.Fprintf(w, "%s", string(data))
}
