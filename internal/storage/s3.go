// Package storage incapsula l'accesso al bucket S3 usato per i file caricati
// dagli utenti (per ora: immagini avatar). Il bucket resta privato: i file non
// sono leggibili pubblicamente da S3, ma vengono serviti dall'app tramite un
// handler proxy (vedi handlers/media.go), così non serve configurare l'accesso
// pubblico del bucket e le credenziali restano l'unica via d'accesso.
package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	client *s3.Client
	bucket string
)

// Init inizializza il client S3 leggendo le credenziali e la configurazione
// dalle variabili d'ambiente standard dell'SDK AWS:
//
//	AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY  (credenziali IAM)
//	AWS_REGION                                (regione del bucket, es. eu-north-1)
//	AWS_S3_BUCKET_NAME                         (nome del bucket)
//
// Se AWS_S3_BUCKET_NAME non è impostata, lo storage resta disattivato
// (Configured() == false) e l'app continua a funzionare con i soli avatar
// generati / URL esterni, senza permettere l'upload di file.
func Init(ctx context.Context) error {
	bucket = os.Getenv("AWS_S3_BUCKET_NAME")
	if bucket == "" {
		return nil // storage non configurato: feature disattivata, non è un errore
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "eu-north-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return err
	}
	client = s3.NewFromConfig(cfg)
	return nil
}

// Configured indica se lo storage S3 è attivo (bucket impostato e client pronto).
func Configured() bool { return client != nil && bucket != "" }

// Upload carica un oggetto nel bucket con la chiave e il content-type indicati.
func Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	if !Configured() {
		return errors.New("storage S3 non configurato")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

// Object rappresenta un oggetto scaricato da S3: il corpo (da chiudere a cura
// del chiamante) e il content-type salvato all'upload.
type Object struct {
	Body        io.ReadCloser
	ContentType string
}

// Download recupera un oggetto dal bucket.
func Download(ctx context.Context, key string) (*Object, error) {
	if !Configured() {
		return nil, errors.New("storage S3 non configurato")
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	return &Object{Body: out.Body, ContentType: ct}, nil
}
