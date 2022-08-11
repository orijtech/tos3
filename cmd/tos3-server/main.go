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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/odeke-em/go-uuid"
	"github.com/orijtech/otils"
	"github.com/orijtech/tos3"
)

var s3Client *s3.S3
var defaultPath string
var defaultBucket string

func pathOf(req *tos3.Request) string {
	path := strings.TrimSpace(req.Path)
	if path == "" {
		return uuid.NewRandom().String()
	}
	return path
}

func init() {
	envCred := credentials.NewEnvCredentials()
	config := aws.NewConfig().WithCredentials(envCred)
	sess := session.Must(session.NewSession(config))
	s3Client = s3.New(sess)
}

func parseReq(r io.Reader) (*tos3.Request, error) {
	blob, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Got: %s\n", blob)
	req := new(tos3.Request)
	if err := json.Unmarshal(blob, req); err != nil {
		return nil, err
	}

	return req, nil
}

func quotaCheck(req *tos3.Request) {
	// TODO: Implement me.
}

func uploadIt(rw http.ResponseWriter, req *http.Request) {
	ctx, span := trace.StartSpan(req.Context(), "uploadIt")
	defer span.End()
	defer req.Body.Close()

	hdr := req.Header
	payloadJSON := hdr.Get("payload_json")
	if len(payloadJSON) == 0 {
		http.Error(rw, `expected "payload_json" in the request headers`, http.StatusBadRequest)
		return
	}

	preq, err := parseReq(strings.NewReader(payloadJSON))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	// Now set the defaultBucket
	// TODO: Allow paying customers to upload
	preq.Bucket = defaultBucket
	preq.Path = pathOf(preq)

	if err := preq.Validate(); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	// Firstly detect the content-type.
	var contentType string
	// We've got to sniff the contentType first
	sniffBytes := make([]byte, 512)
	if n, err := io.ReadAtLeast(req.Body, sniffBytes, 1); err == nil && n > 0 {
		contentType = http.DetectContentType(sniffBytes[:n])
	}
	preq.ContentType = contentType

	// We need to have an io.ReadSeeker in order
	// for the S3 upload to succeed.
	tmpf, err := os.CreateTemp(os.TempDir(), "upload")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tmpf.Close()

	// Firstly write back the sniff bytes.
	if _, err := tmpf.Write(sniffBytes); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(tmpf, req.Body); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := tmpf.Seek(0, 0); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	preq.S3Client = s3Client
	resp, err := preq.FUploadToS3(ctx, tmpf)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	blob, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Write(blob)
}

func main() {
	var port int
	ocAgentAddress := flag.String("ocagent-addr", "", "The address to connect to the OpenCensus/Telemetry CAgent")
	flag.IntVar(&port, "port", 8833, "the port to run it on")
	flag.StringVar(&defaultBucket, "default-bucket", "tatan", "the default bucket to use")
	flag.StringVar(&defaultPath, "common-io", "tatan", "the default path to use")
	flag.Parse()

	oce, err := ocagent.NewExporter(
		ocagent.WithInsecure(),
		ocagent.WithServiceName(otils.EnvOrAlternates("TO_S3_SERVER_SERVICE_NAME", "cmd/tos3")),
		ocagent.WithAddress(*ocAgentAddress),
	)
	if err != nil {
		panic(err)
	}
	defer oce.Stop()

	trace.RegisterExporter(oce)
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", uploadIt)
	ocmux := &ochttp.Handler{
		Handler: mux,
	}

	addr := fmt.Sprintf(":%d", port)
	log.Printf("running on %v\n", addr)

	if err := http.ListenAndServe(addr, ocmux); err != nil {
		log.Fatal(err)
	}
}
