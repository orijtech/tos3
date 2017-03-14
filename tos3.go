package tos3

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/odeke-em/go-uuid"
	"github.com/odeke-em/tmpfile"
)

type AuthInfo struct {
	AccessKeyId string `json:"akid,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	Message     string `json:"message,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

type GeoInfo struct {
	Latitude  float32 `json:"lat,omitempty"`
	Longitude float32 `json:"lon,omitempty"`
	Radius    float32 `json:"radius,omitempty"`

	X float32 `json:"x,omitempty"`
	Y float32 `json:"y,omitempty"`
}

type MetaInfo struct {
	Name string     `json:"name,omitempty"`
	Time *time.Time `json:"time,omitempty"`
	Tags []string   `json:"tags,omitempty"`

	Comments []string `json:"comments,omitempty"`

	Other map[string]interface{} `json:"other,omitempty"`
}

type Request struct {
	Private bool   `json:"private,omitempty"`
	Bucket  string `json:"bucket,omitempty"`
	Path    string `json:"path,omitempty"`
	URL     string `json:"url,omitempty"`
	Name    string `json:"name,omitempty"`

	ContentType string `json:"content_type,omitempty"`

	S3Client      *s3.S3 `json:"-"`
	ContentLength int64  `json:"-"`

	AuthInfo *AuthInfo `json:"auth_info,omitempty"`
	GeoInfo  *GeoInfo  `json:"geo_info,omitempty"`
	MetaInfo *MetaInfo `json:"meta_info,omitempty"`
}

type Response struct {
	RequestId   string `json:"request_id,omitempty"`
	ETag        string `json:"etag,omitempty"`
	VersionId   string `json:"version_id,omitempty"`
	URL         string `json:"url,omitempty"`
	Bucket      string `json:"bucket,omitempty"`
	Name        string `json:"name,omitempty"`
	MD5Checksum string `json:"md5,omitempty"`
}

var (
	errEmptyURL    = errors.New("expecting a non-empty URL")
	errEmptyPath   = errors.New("expecting a non-empty path")
	errEmptyBucket = errors.New("expecting a non-empty bucket")
)

func (req *Request) Validate() error {
	if req == nil || req.URL == "" {
		return errEmptyURL
	}
	if req.Path == "" {
		return errEmptyPath
	}
	if req.Bucket == "" {
		return errEmptyBucket
	}
	return nil
}

var errUnimplemented = errors.New("unimplemented")

func (req *Request) Search() (*Response, error) {
	// TODO: The search route
	return nil, errUnimplemented
}

func (req *Request) Delete() (*Response, error) {
	// TODO: The delete route
	return nil, errUnimplemented
}

func (req *Request) FUploadToS3(body io.ReadSeeker) (*Response, error) {
	// TODO: See if this content was already uploaded,
	// bearing the same path and MD5Checksum then
	// make that a retrieval instead of an upload
	// to conserve bandwidth
	pin := &s3.PutObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Path),
		Body:   body,
	}

	if ct := req.ContentType; ct != "" {
		pin.ContentType = aws.String(ct)
	}

	if !req.Private {
		pin.ACL = aws.String("public-read")
	}

	if req.ContentLength >= 1 {
		pin.ContentLength = &req.ContentLength
	}

	fmt.Printf("pin: #%v\n", pin)

	pout, err := req.S3Client.PutObject(pin)
	if err != nil {
		return nil, err
	}

	resp := &Response{
		Bucket:    *pin.Bucket,
		Name:      *pin.Key,
		URL:       makeS3URL(pin),
		ETag:      *pout.ETag,
		VersionId: *pout.VersionId,
	}

	return resp, nil
}

func (req *Request) UploadToS3() (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	requestId := uuid.NewRandom().String()

	res, err := http.Get(req.URL)
	if err != nil {
		return nil, err
	}
	var abort func() error = res.Body.Close

	if !statusOK(res.StatusCode) {
		_ = abort()
		return nil, fmt.Errorf("%s", res.Status)
	}

	var contentType string
	// We've got to sniff the contentType first
	sniffBytes := make([]byte, 512)
	_, err = io.ReadAtLeast(res.Body, sniffBytes, 1)
	if err == nil {
		contentType = http.DetectContentType(sniffBytes)
	}

	// Otherwise we have to bite the bullet here
	// and write to tmpfile then cleanup after
	ctx := &tmpfile.Context{
		Suffix: fmt.Sprintf("tos3-%s", requestId),
	}
	tmpf, err := tmpfile.New(ctx)
	if err != nil {
		_ = abort()
		return nil, err
	}
	oldAbort := abort
	abort = func() error {
		_ = oldAbort()
		return tmpf.Done()
	}
	defer abort()

	var md5Checksum string
	md5H := md5.New()
	body := io.MultiReader(bytes.NewReader(sniffBytes), res.Body)
	tr := io.TeeReader(body, md5H)
	n, err := io.Copy(tmpf, tr)
	if err != nil {
		return nil, err
	}
	req.ContentType = contentType
	req.ContentLength = n
	s3Res, err := req.FUploadToS3(tmpf)
	if err != nil {
		return nil, err
	}

	md5Checksum = fmt.Sprintf("%x", md5H.Sum(nil))
	s3Res.MD5Checksum = md5Checksum
	return s3Res, nil
}

func statusOK(code int) bool { return code >= 200 && code <= 299 }

func makeS3URL(pin *s3.PutObjectInput) string {
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", *pin.Bucket, *pin.Key)
}
