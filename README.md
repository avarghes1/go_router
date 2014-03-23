Simple Go Rest Router
=====================

The go_router is a simple rest based router. The supported HTTP methods are GET, POST and DELETE.

The url path has to be in the form `/version/resource/handler/param-name/param-value`.
Json is the supported response type. The router also supports filters for pre and post dispatch process.

Example Controller:
---

```
func Retrieve(req go_router.Request) (string, error) {
    fmt.Println(req["id"].Int())
    return "test", nil
}
```

Setting up a route:
---

```
go_router.RegisterRoute("GET", "/v1/test/retrieve/id/{Int}", test.Retrieve)
```
