## sync-message-server

sync-message-server converts Plivo's messaging (SMS, MMS) API from async to sync.

### Usage

```sh
$ # Edit config.go
$ go build
$ ./sync-message-server
2021/08/19 23:13:35 started reverse proxy; addr=:8090
2021/08/19 23:13:35 started callback server; addr=:8091
2021/08/19 23:37:37 responding; body={"message_uuid":"54ada4a2-0118-11ec-8ca5-0242ac110006","message_state":"sent","message_time":"2021-08-19 18:07:35.315189","sent_time":"2021-08-19 18:07:37.280569","total_rate":"0.03800","total_amount":"0.03800","units":1,"error_code":""}
```

Example request and response:

```sh
$ curl -s -u MAXXXXXXXXXXXXXXXXXX:YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY -X POST http://localhost:8090/v1/Account/MAXXXXXXXXXXXXXXXXXX/Message/ -H 'content-type: application/json' -d '{
  "src" : "+1 xxx-xxx-xxx",
  "dst" :"+91 xxx-xxx-xxx",
  "text" : "Hello World!"
}' | python -m json.tool
```
```json
{
    "error_code": "",
    "message_state": "sent",
    "message_time": "2021-08-19 18:07:35.315189",
    "message_uuid": "54ada4a2-0118-11ec-8ca5-0242ac110006",
    "sent_time": "2021-08-19 18:07:37.280569",
    "total_amount": "0.03800",
    "total_rate": "0.03800",
    "units": 1
}
```
