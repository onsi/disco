package s3db

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var TIMEOUT = time.Second * 10
var ErrObjectNotFound = errors.New("object not found")
var ErrTimeout = errors.New("timed out")

type S3DBInt interface {
	FetchObject(key string) ([]byte, error)
	PutObject(key string, data []byte) error
}

type S3DB struct {
	svc *s3.S3

	bucket string
	env    string
}

func NewS3DB(accessKey, secretKey, region, bucket, env string) (*S3DB, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	})
	if err != nil {
		return nil, err
	}
	return &S3DB{
		svc:    s3.New(sess),
		bucket: bucket,
		env:    env,
	}, nil
}

func (s3db *S3DB) FetchObject(okey string) ([]byte, error) {
	key := s3db.env + "/" + okey
	ctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	resp, err := s3db.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3db.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchKey {
				return nil, ErrObjectNotFound
			} else if aerr.Code() == request.CanceledErrorCode {
				return nil, ErrTimeout
			}
		}
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s3db *S3DB) PutObject(okey string, data []byte) error {
	key := s3db.env + "/" + okey
	ctx, cancel := context.WithTimeout(context.Background(), TIMEOUT)
	defer cancel()
	_, err := s3db.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s3db.bucket),
		Key:    aws.String(key),
		Body:   aws.ReadSeekCloser(bytes.NewReader(data)),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == request.CanceledErrorCode {
				return ErrTimeout
			}
		}
	}
	return err
}
