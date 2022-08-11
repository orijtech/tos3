// Copyright 2022 Orijtech Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package example_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/orijtech/tos3"
)

func ExampleUploadFileToS3() {
	f, err := os.Open(os.Getenv("TO_S3_UPLOAD_FILE"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	req, err := http.NewRequest("POST", "http://localhost:8833/", f)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "file/octet-stream")
	payloadBody := tos3.Request{
		Path: "926abe55d7ac4c8caa1cce89695650c5/profile_pic",
		Name: "profile_pic",
	}
	payloadJSON, err := json.Marshal(payloadBody)
	if err != nil {
		panic(err)
	}
	req.Header.Add("payload_json", string(payloadJSON))

	reqBlob, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		panic(err)
	}
	println(string(reqBlob))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		panic(res.Status)
	}
	defer res.Body.Close()
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	println(string(blob))
	// Output:
	//   This
}
