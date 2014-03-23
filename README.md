Simple Go Rest Router
=====================

The go_router is a simple rest based router. The supported HTTP methods are GET, POST and DELETE.

The url path has to be in the form `/version/resource/handler/param-name/param-value`.
Json is the supported response type. The router also supports filters for pre and post dispatch process.

Installation:
---

`go get github.com/avarghes1/go_router/router`

Import:
---

`import github.com/avarghes1/go_router_router`

Wire up the router:
---

```
http.HandleFunc("/", go_router.Dispatch)
http.ListenAndServe(":8080", nil)
```

Setting up a route:
---

`go_router.RegisterRoute("GET", "/v1/test/retrieve/id/{Int}", test.Retrieve)`

Example Controller:
---

```
func Retrieve(req go_router.Request) (string, error) {
    fmt.Println(req["id"].Int())
    return "test", nil
}
```
