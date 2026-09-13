// Package checkeast runs a configuration diff and archives it to S3-compatible
// object storage, keeping an auditable, timestamped record of every diff. It
// reuses the existing diff pipeline (annet.Service.ExecuteCommand) and only
// adds the archival step, so diffing is never re-implemented here.
package checkeast

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"annet-oil/internal/config"
	"annet-oil/internal/logging"
)

// objectPutter uploads a single object. It is the seam the Store's archiving
// logic depends on, so tests can inject a fake without touching S3.
type objectPutter interface {
	put(ctx context.Context, key string, body []byte, contentType string, gzipped bool) (int, error)
}

// objectReader lists and fetches archived objects. It is a separate seam from
// objectPutter so the read-back path can be tested without touching S3.
type objectReader interface {
	list(ctx context.Context, prefix string) ([]ObjectInfo, error)
	get(ctx context.Context, key string) ([]byte, string, error)
}

// ObjectInfo describes one archived object as listed from S3.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Store archives diffs to S3-compatible storage. A nil *Store means "disabled";
// callers must null-check it (same convention as logging.NewS3Uploader).
type Store struct {
	putter objectPutter
	reader objectReader
	bucket string
	prefix string
}

// New returns (nil, nil) when the store is disabled or misconfigured, so callers
// can simply check `if store != nil`. When a retention is configured (default 3
// days) it applies a best-effort bucket lifecycle rule scoped to the prefix.
func New(cfg config.S3StoreConfig) (*Store, error) {
	if !cfg.Enabled || cfg.Bucket == "" {
		return nil, nil
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // required by MinIO / S3-compatible stores
		}
	})

	store := &Store{
		putter: &s3Putter{client: client, bucket: cfg.Bucket},
		reader: &s3Reader{client: client, bucket: cfg.Bucket},
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}

	// Apply the retention lifecycle (default 3 days). Best-effort: a store that
	// cannot manage lifecycle (e.g. restricted IAM) still archives diffs.
	if days := cfg.EffectiveRetentionDays(); days > 0 {
		if err := ensureLifecycle(ctx, client, cfg.Bucket, cfg.Prefix, days); err != nil {
			logging.Warn("checkeast: failed to apply S3 retention lifecycle",
				"bucket", cfg.Bucket, "prefix", cfg.Prefix, "days", days, "error", err)
		} else {
			logging.Info("checkeast: applied S3 retention lifecycle",
				"bucket", cfg.Bucket, "prefix", cfg.Prefix, "days", days)
		}
	}

	return store, nil
}

// ensureLifecycle installs an idempotent lifecycle rule that expires archived
// diffs under prefix after the given number of days.
func ensureLifecycle(ctx context.Context, client *s3.Client, bucket, prefix string, days int) error {
	rule := s3types.LifecycleRule{
		ID:         aws.String("annet-oil-checkeast-retention"),
		Status:     s3types.ExpirationStatusEnabled,
		Filter:     &s3types.LifecycleRuleFilter{Prefix: aws.String(prefix)},
		Expiration: &s3types.LifecycleExpiration{Days: aws.Int32(int32(days))},
	}

	_, err := client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{
			Rules: []s3types.LifecycleRule{rule},
		},
	})
	return err
}

// s3Putter is the production objectPutter backed by the AWS S3 client.
type s3Putter struct {
	client *s3.Client
	bucket string
}

// put uploads one object, gzip-compressing the body first when gzipped is true,
// and returns the number of bytes actually written to S3.
func (p *s3Putter) put(ctx context.Context, key string, body []byte, contentType string, gzipped bool) (int, error) {
	data := body
	var contentEncoding *string
	if gzipped {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(body); err != nil {
			gz.Close()
			return 0, fmt.Errorf("gzip: %w", err)
		}
		if err := gz.Close(); err != nil {
			return 0, fmt.Errorf("gzip close: %w", err)
		}
		data = buf.Bytes()
		contentEncoding = aws.String("gzip")
	}

	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(p.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(data),
		ContentType:     aws.String(contentType),
		ContentEncoding: contentEncoding,
	})
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// s3Reader is the production objectReader backed by the AWS S3 client.
type s3Reader struct {
	client *s3.Client
	bucket string
}

// list returns every object under prefix, following pagination.
func (rd *s3Reader) list(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	p := s3.NewListObjectsV2Paginator(rd.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(rd.bucket),
		Prefix: aws.String(prefix),
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.Contents {
			info := ObjectInfo{Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size)}
			if o.LastModified != nil {
				info.LastModified = *o.LastModified
			}
			out = append(out, info)
		}
	}
	return out, nil
}

// get downloads one object, transparently gunzipping gzip-encoded payloads
// (the combined diff.json.gz archives).
func (rd *s3Reader) get(ctx context.Context, key string) ([]byte, string, error) {
	obj, err := rd.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(rd.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	defer obj.Body.Close()

	data, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := aws.ToString(obj.ContentType)
	if aws.ToString(obj.ContentEncoding) == "gzip" || strings.HasSuffix(key, ".gz") {
		if zr, zerr := gzip.NewReader(bytes.NewReader(data)); zerr == nil {
			if dec, derr := io.ReadAll(zr); derr == nil {
				data = dec
			}
			zr.Close()
		}
		if strings.HasSuffix(key, ".json.gz") {
			contentType = "application/json"
		}
	}
	return data, contentType, nil
}

// StoredDiff is one archived per-host diff, parsed from its S3 object key.
type StoredDiff struct {
	Key          string    `json:"key"`
	Host         string    `json:"host"`
	RunID        string    `json:"run_id"`
	Timestamp    time.Time `json:"timestamp"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// List returns the archived per-host diffs (the "{host}.diff" objects), newest
// first. When host is non-empty only diffs for that host are returned. The
// combined diff.json.gz archives are skipped — they are addressable via Get.
func (s *Store) List(ctx context.Context, host string) ([]StoredDiff, error) {
	if s == nil {
		return nil, nil
	}
	objs, err := s.reader.list(ctx, s.prefix)
	if err != nil {
		return nil, err
	}
	var diffs []StoredDiff
	for _, o := range objs {
		if !strings.HasSuffix(o.Key, ".diff") {
			continue
		}
		d := parseDiffKey(s.prefix, o)
		if host != "" && !strings.EqualFold(d.Host, host) {
			continue
		}
		diffs = append(diffs, d)
	}
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Timestamp.After(diffs[j].Timestamp)
	})
	return diffs, nil
}

// Get returns the raw content (and content type) of an archived object by key.
// The key must live under the store's prefix, guarding against reads of
// arbitrary bucket objects.
func (s *Store) Get(ctx context.Context, key string) ([]byte, string, error) {
	if s == nil {
		return nil, "", fmt.Errorf("checkeast store is disabled")
	}
	if s.prefix != "" && !strings.HasPrefix(key, s.prefix) {
		return nil, "", fmt.Errorf("key %q is outside the archive prefix", key)
	}
	return s.reader.get(ctx, key)
}

// parseDiffKey extracts host / runID / timestamp from a per-host diff key of the
// form "{prefix}YYYY/MM/DD/{runID}/{host}.diff". The runID begins with a UTC
// timestamp (20060102-150405), optionally suffixed with a host token.
func parseDiffKey(prefix string, o ObjectInfo) StoredDiff {
	d := StoredDiff{Key: o.Key, Size: o.Size, LastModified: o.LastModified}
	rel := strings.TrimPrefix(o.Key, prefix)
	parts := strings.Split(rel, "/")
	if len(parts) >= 2 {
		d.Host = strings.TrimSuffix(parts[len(parts)-1], ".diff")
		d.RunID = parts[len(parts)-2]
	}
	if len(d.RunID) >= 15 {
		if t, err := time.Parse("20060102-150405", d.RunID[:15]); err == nil {
			d.Timestamp = t.UTC()
		}
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = o.LastModified
	}
	return d
}
