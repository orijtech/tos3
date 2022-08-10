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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

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
	resp, err := preq.UploadToS3()
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
	flag.IntVar(&port, "port", 8833, "the port to run it on")
	flag.StringVar(&defaultBucket, "default-bucket", "tatan", "the default bucket to use")
	flag.StringVar(&defaultPath, "common-io", "tatan", "the default path to use")
	flag.Parse()

	http.HandleFunc("/", uploadIt)

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	log.Printf("running on %v\n", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
