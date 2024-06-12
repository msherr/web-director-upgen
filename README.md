# web-director

## Compiling

```bash
go build ./cmd/server

```

## Testing

```
curl --header "Content-Type: application/json" \
  -H "X-Session-Token: micah1" \
  --request POST \
  --data '{"username":"xyz","password":"xyz"}' \
  https://localhost:8888/exit
```