// Package storage wraps Azure Blob storage for image uploads.
//
// Architecture: backend issues a short-lived SAS (Shared Access Signature)
// token. The browser uploads the image bytes directly to Blob using that
// token, bypassing our Go service. This keeps large file transfers off the
// App Service CPU/RAM and works well even on B1.
package storage

import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

type Client struct {
	accountName string
	container   string
	cred        *azblob.SharedKeyCredential
}

// NewClient parses an Azure storage connection string of the form:
//
//	DefaultEndpointsProtocol=https;AccountName=...;AccountKey=...;EndpointSuffix=core.windows.net
//
// and returns a client scoped to the given container (e.g. "images").
func NewClient(connectionString, container string) (*Client, error) {
	parts := parseConnString(connectionString)
	name := parts["AccountName"]
	key := parts["AccountKey"]
	if name == "" || key == "" {
		return nil, fmt.Errorf("invalid storage connection string (missing AccountName or AccountKey)")
	}
	if container == "" {
		return nil, fmt.Errorf("storage container name is required")
	}

	cred, err := azblob.NewSharedKeyCredential(name, key)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}
	return &Client{accountName: name, container: container, cred: cred}, nil
}

func parseConnString(s string) map[string]string {
	m := make(map[string]string)
	for _, kv := range strings.Split(s, ";") {
		if kv == "" {
			continue
		}
		if i := strings.Index(kv, "="); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

// SASResult is what the client gets back from a SAS request.
type SASResult struct {
	UploadURL string    `json:"upload_url"` // PUT image bytes here
	PublicURL string    `json:"public_url"` // store this in markdown
	BlobName  string    `json:"blob_name"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateUploadSAS builds a write-only, short-lived SAS for one specific
// blob. Browser PUTs the image bytes to UploadURL with headers:
//
//	x-ms-blob-type: BlockBlob
//	Content-Type:   <the file's mime type>
//
// PublicURL is the same URL without the SAS query — that's what we embed
// in markdown.
func (c *Client) GenerateUploadSAS(blobName string, ttl time.Duration) (*SASResult, error) {
	now := time.Now().UTC()
	expiry := now.Add(ttl)

	values := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     now.Add(-2 * time.Minute), // small clock-skew buffer
		ExpiryTime:    expiry,
		Permissions:   (&sas.BlobPermissions{Create: true, Write: true}).String(),
		ContainerName: c.container,
		BlobName:      blobName,
	}

	qp, err := values.SignWithSharedKey(c.cred)
	if err != nil {
		return nil, fmt.Errorf("sign sas: %w", err)
	}

	publicURL := fmt.Sprintf(
		"https://%s.blob.core.windows.net/%s/%s",
		c.accountName, c.container, blobName,
	)
	uploadURL := publicURL + "?" + qp.Encode()

	return &SASResult{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		BlobName:  blobName,
		ExpiresAt: expiry,
	}, nil
}
