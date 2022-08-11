package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/odeke-em/go-uuid"
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
	creds, err := envCred.Get()
	if err != nil {
		log.Fatalf("failed to retrieve environment credentials: %v", err)
	}
	config := aws.NewConfig().WithCredentials(envCred)
	fmt.Printf("creds: %#v\n", creds)
	sess := session.Must(session.NewSession(config))
	s3Client = s3.New(sess)
}

func parseReq(r io.Reader) (*tos3.Request, error) {
	blob, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	req := new(tos3.Request)
	if err := json.Unmarshal(blob, req); err != nil {
		return nil, err
	}

	return req, nil
}

func quotaCheck(req *tos3.Request) {
}

func uploadIt(rw http.ResponseWriter, req *http.Request) {
        ctx, span := trace.StartSpan(req.Context(), "uploadIt")
        defer span.End()
	defer req.Body.Close()

	preq, err := parseReq(req.Body)
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

	preq.S3Client = s3Client
	resp, err := preq.UploadToS3(ctx)
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
		ocagent.WithServiceName("cmd/tos3"),
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
