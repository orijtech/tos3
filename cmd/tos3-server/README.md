## tos3-server

The / endpoint expects an HTTP POST request with:

* the file to be uploaded as the body of the POST request
* "payload_json" as a header value with the JSON value of a Request, whose contents MUST be JSON serialized

### Request

Field name|Type/Validation|Required|Comment
---|---|---|---
private|boolean|No|If set will set the file permissions to private, so by default public files
bucket|string|If not configured in the environment, should be accessible by the environment AWS credentials|
path|string|Yes|the path offset from the bucket that'll finally appear in the URL
name|string|Yes|representative name for the file

If for example we ran the example in examples/upload_test.go or just sent this HTTP POST request
```HTTP
POST / HTTP/1.1
Host: localhost:8833
User-Agent: Go-http-client/1.1
Transfer-Encoding: chunked
Content-Type: file/octet-stream
Payload_json: {"path":"926abe55d7ac4c8caa1cce89695650c5/profile_pic","name":"profile_pic"}
Accept-Encoding: gzip
```

### Response
This spits out this response

```json
{
  "etag": "\"5b6c7b4aed837e8ed0f9950564a10b32\"",
  "version_id": "73WpSQNRjQYhGWplXkml9Kz2HhcTJxr6",
  "url": "https://tatan.s3.amazonaws.com/926abe55d7ac4c8caa1cce89695650c5/profile_pic",
  "bucket": "tatan",
  "name": "926abe55d7ac4c8caa1cce89695650c5/profile_pic"
}
