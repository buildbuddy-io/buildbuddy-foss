package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Note: the doc generator script (`generate_docs.py`) in this directory
// generates documentation from this file at build time.
//
// The doc generator treats the following things specially:
//
// * Metric definitions (promauto.New...): these will generate a markdown block
//   with a nicely formatted metric entry, including the name, type, help
//   text, and label values, along with the help text for each label.
//
// * Label constants: the comment above each of these constants is treated
//   as the help text for the metric label (in the generated metric definition
//   described above).
//
// * Comment blocks starting with "// #" (like "// ## Section 1" introducing
//   a new metrics section, or "// #### Examples" for an example prometheus
//   query): these are inserted directly into the documentation as markdown.

// Label constants.
const (
	// TODO(bduffany): Migrate StatusLabel usages to StatusHumanReadableLabel

	// Status code as defined by [grpc/codes](https://godoc.org/google.golang.org/grpc/codes#Code).
	// This is a numeric value; any non-zero code indicates an error.
	StatusLabel = "status"

	// Status code as defined by [grpc/codes](https://godoc.org/google.golang.org/grpc/codes#Code)
	// in human-readable format, such as "OK" or "NotFound".
	StatusHumanReadableLabel = "status"

	// Invocation status: `success`, `failure`, `disconnected`, or `unknown`.
	InvocationStatusLabel = "invocation_status"

	// Cache type: `action` for action cache, `cas` for content-addressable storage.
	CacheTypeLabel = "cache_type"

	// Cache event type: `hit`, `miss`, or `upload`.
	CacheEventTypeLabel = "cache_event_type"

	// Cache name: Custom name to describe the cache, like "pebble-cache".
	CacheNameLabel = "cache_name"

	// Process exit code of an executed action.
	ExitCodeLabel = "exit_code"

	// Generic label to describe the stage the metric is capturing
	Stage = "stage"

	// SQL query before substituting template parameters.
	SQLQueryTemplateLabel = "sql_query_template"

	// `gcs` (Google Cloud Storage), `aws_s3`, or `disk`.
	BlobstoreTypeLabel = "blobstore_type"

	// Status of the database connection: `in_use` or `idle`
	SQLConnectionStatusLabel = "connection_status"

	// SQL DB replica role: `primary` for read+write replicas, or
	// `read_replica` for read-only DB replicas.
	SQLDBRoleLabel = "sql_db_role"

	// HTTP route before substituting path parameters
	// (`/invocation/:id`, `/settings`, ...)
	HTTPRouteLabel = "route"

	// HTTP method: `GET`, `POST`, ...
	HTTPMethodLabel = "method"

	// HTTP response code: `200`, `302`, `401`, `404`, `500`, ...
	HTTPResponseCodeLabel = "code"

	// Host (without any port number) for outgoing HTTP requests
	HTTPHostLabel = "host"

	// Cache backend: `gcs` (Google Cloud Storage), `aws_s3`, or `redis`.
	CacheBackendLabel = "backend"

	// Cache tier: `memory` or `cloud`. This label can be used to write Prometheus
	// queries that don't break if the cache backend is swapped out for
	// a different backend.
	CacheTierLabel = "tier"

	// Command provided to the Bazel daemon: `run`, `test`, `build`, `coverage`, `mobile-install`, ...
	BazelCommand = "bazel_command"

	// Exit code of a completed bazel command
	BazelExitCode = "bazel_exit_code"

	// Bazel version
	BazelVersion = "bazel_version"

	// Executed action stage. Action execution is split into stages corresponding to
	// the timestamps defined in
	// [`ExecutedActionMetadata`](https://github.com/buildbuddy-io/buildbuddy/blob/fb2e3a74083d82797926654409dc3858089d260b/proto/remote_execution.proto#L797):
	// `queued`, `input_fetch`, `execution`, and `output_upload`. An additional stage,
	// `worker`, includes all stages during which a worker is handling the action,
	// which is all stages except the `queued` stage.
	ExecutedActionStageLabel = "stage"

	// System resource: "cpu", "memory", or "io".
	PSIResourceLabel = "resource"

	// Pressure stall type: "some" (task is partially stalled on the resource)
	// or "full" (task is completely stalled on the resource).
	PSIStallTypeLabel = "stall_type"

	// Type of event sent to BuildBuddy's webhook handler: `push` or
	// `pull_request`.
	WebhookEventName = "event"

	// Status of the recycle runner request: `hit` if the executor assigned a
	// recycled runner to the action; `miss` otherwise.
	RecycleRunnerRequestStatusLabel = "status"

	// Reason for a runner not being added to the runner pool.
	RunnerPoolFailedRecycleReason = "reason"

	// Effective workload isolation type used for an executed task, such as
	// "docker", "podman", "firecracker", or "none".
	IsolationTypeLabel = "isolation"

	// Group (organization) ID associated with the request.
	GroupID = "group_id"

	// OS associated with the request.
	OS = "os"

	// CPU architecture associated with the request.
	Arch = "arch"

	// The name used to identify the type of an unexpected event.
	EventName = "name"

	// The ID of the disk cache partition this event applied to.
	PartitionID = "partition_id"

	// Status of the file cache request: `hit` if found in cache, `miss` otherwise.
	FileCacheRequestStatusLabel = "status"

	// Status of the task size read request: `hit`, `miss`, or `error`.
	TaskSizeReadStatusLabel = "status"

	// Status of the task size write request: `ok`, `missing_stats` or `error`.
	TaskSizeWriteStatusLabel = "status"

	// The namespace that is protected by quota manager.
	QuotaNamespace = "quota_namespace"

	// The key used for quota accounting, either a group ID or an IP address.
	QuotaKey = "quota_key"

	// Describes the type of cache request
	CacheRequestType = "type"

	// Origin of the cache request (for usage tracking): should be "internal" or "external"
	CacheRequestOrigin = "origin"

	// Describes the name of the server that handles a client request, such as "byte_stream_server" or "cas_server"
	ServerName = "server_name"

	// Describes the type of compression
	CompressionType = "compression"

	// The name of the table in Clickhouse
	ClickhouseTableName = "clickhouse_table_name"

	// Status of the Clickhouse operation: `ok`, `error`.
	ClickhouseStatusLabel = "status"

	// The ID of a raft nodehost.
	RaftNodeHostIDLabel = "node_host_id"

	// The range ID of a raft region.
	RaftRangeIDLabel = "range_id"

	// The type of nodehost method: "SyncPropose", "SyncRead"
	RaftNodeHostMethodLabel = "nodehost_method"

	// The error returned by dragonboat library
	RaftDragonboatError = "dragonboat_error"

	// The type of raft move `add`, or `remove`.
	RaftMoveLabel = "move_type"

	// The type of lease action `Acquire`, `Drop`.
	RaftLeaseActionLabel = "lease_action"

	// Raft RangeCache event type: `hit`, `miss`, or `update`.
	RaftRangeCacheEventTypeLabel = "rangecache_event_type"

	// Raft Listener Event Type, such as "LeaderUpdated". These are events sent
	// by dragonboat library.
	RaftListenerEventType = "listener_event"

	// The ID of a raft listener
	RaftListenerID = "listener_id"

	// The source of the event broadcast.
	RaftEventBroadcaster = "event_broadcast_source"

	// Raft Event Type, such as "range-removed", "range-usage-updated"
	RaftEventType = "raft_event"

	// Raft Driver Action
	RaftDriverAction = "driver_action"

	// Raft Driver Requeue Type
	RaftDriverRequeueType = "driver_requeue_type"

	// Raft Replica State: staging, removing
	RaftReplicaState = "replica_state"

	// The status of processing txn record: "success" or "failure"
	RaftTxnRecordProcessStatus = "txn_record_process_status"

	// Binary version. Example: `v2.0.0`.
	VersionLabel = "version"

	// Binary git commit SHA. Example: `4bd7046417608d785094aa5ec7aa009a9ae53753`
	CommitLabel = "commit"

	// Whether or not the API Key lookup hit the in memory
	// cache or not: "cache_hit", "cache_miss" or "invalid_key".
	APIKeyLookupStatus = "status"

	// Pebble DB compaction type.
	CompactionType = "compaction_type"

	// Pebble DB level number.
	PebbleLevel = "level"

	// Pebble DB operation type.
	PebbleOperation = "pebble_op"

	// Pebble DB ID
	PebbleID = "pebble_id"

	// Name of service the health check is running for (Ex "distributed_cache" or "sql_primary").
	HealthCheckName = "health_check_name"

	// Container image tag.
	ContainerImageTag = "container_image_tag"

	// SociArtifactStore.GetArtifacts outcome tag.
	GetSociArtifactsOutcomeTag = "get_soci_artifacts_outcome_tag"

	// The TreeCache status: hit/miss/invalid_entry.
	TreeCacheLookupStatus = "status"

	// The TreeCache set status: success/deadline_exceeded/other_error
	TreeCacheSetStatus = "status"

	// The TreeCache split lookup status: hit/miss/failure
	TreeCacheSplitLookupStatus = "status"

	// Where a directory from a GetTree request was found: one of "uncached", "filecache", "remote"
	GetTreeLookupLocation = "location"

	// The Lookaside cache status: hit/miss.
	LookasideCacheLookupStatus = "status"

	// The reason an item was evicted from the lookaside cache.
	// One of: "expired" or "size"
	LookasideCacheEvictionReason = "eviction_reason"

	// Distributed cache operation name, such as "FindMissing" or "Get".
	DistributedCacheOperation = "op"

	// ContentAddressableStorage Server operation: "FindMissingBlobs",
	// "BatchUpdateBlobs", "BatchReadBlobs", or "GetTree".
	CASOperation = "op"

	// Cache lookup result - One of:
	// - "hit"
	// - "miss"
	// - "partial" (for batched RPCs where part of a request was cached)
	// - Or "uncacheable" (for e.g. encrypted resources)
	CacheHitMissStatus = "cache_status"

	// TreeCache directory depth: 0 for the root dir, 1 for a direct child of
	// the root dir, and so on.
	TreeCacheLookupLevel = "level"

	// TreeCache operation "read" or "write"
	TreeCacheOperation = "op"

	// For chunked snapshot files, describes the initialization source of the
	// chunk (Ex. `remote_cache` or `local_filecache`)
	ChunkSource = "chunk_source"

	// For remote execution runners, describes the recycling status (Ex.
	// 'clean' if the runner is not recycled or 'recycled')
	RecycledRunnerStatus = "recycled_runner_status"

	// Name of a file.
	FileName = "file_name"

	// There are a couple of places where we enqueue RPCs to be batched and
	// sent asynchronously: the atime_updater and the hit_tracker_client.
	// This label tracks the outcome of these enqueue operations. One of
	// "enqueued", "duplicate", "dropped_too_many_updates", or
	// "dropped_too_many_batches"
	EnqueueUpdateOutcome = "status"

	// CreatedFromSnapshot indicates if a firecracker execution used a
	// snapshot.
	CreatedFromSnapshot = "created_from_snapshot"

	// Command being run. Specific arguments to the command are omitted to
	// reduce metric cardinality.
	CommandName = "command"

	// Cache proxy request type. If "local_only", indicates the proxy should
	// only use its local cache to fulfil the request. If "default", the proxy
	// should fall back to the remote cache as the source of truth.
	CacheProxyRequestType = "proxy_request_type"

	OCIResourceTypeLabel = "oci_resource_type"

	OpLabel = "op"
)

// Label value constants
const (
	HitStatusLabel         = "hit"
	MissStatusLabel        = "miss"
	PartialStatusLabel     = "partial"
	UncacheableStatusLabel = "uncacheable"

	LocalOnlyCacheProxyRequestLabel = "local_only"
	DefaultCacheProxyRequestLabel   = "default"

	OCIManifestResourceTypeLabel     = "manifest"
	OCIBlobResourceTypeLabel         = "blob"
	OCIBlobMetadataResourceTypeLabel = "blob_metadata"
)

// Other constants
const (
	bbNamespace = "buildbuddy"

	day = 24 * time.Hour
)

// Bucket constants
var (
	// These buckets have very little precision:
	// 0 = 1µs
	// 1 = 10µs
	// 2 = 100µs
	// 3 = 1ms
	// 4 = 10ms
	// 5 = 100ms
	// 6 = 1s
	// 7 = 10s
	// 8 = 1m40s
	// 9 = inf
	// It's probably better to use something specific to the given metric.
	coarseMicrosecondToHour = durationUsecBuckets(1*time.Microsecond, 1*time.Hour, 10)
)

var (
	// ## Invocation build event metrics
	//
	// All invocation metrics are recorded at the _end_ of each invocation.

	InvocationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "count",
		Help:      "The total number of invocations whose logs were uploaded to BuildBuddy.",
	}, []string{
		InvocationStatusLabel,
		BazelExitCode,
		BazelCommand,
	})

	// Number of invocations by Bazel major version.
	InvocationsByBazelVersionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "bazel_version",
		Help:      "The number of invocations by client Bazel version.",
	}, []string{BazelVersion})

	// #### Examples
	//
	// ```promql
	// # Number of invocations per second by invocation status
	// sum by (invocation_status) (rate(buildbuddy_invocation_count[5m]))
	//
	// # Invocation success rate
	// sum(rate(buildbuddy_invocation_count{invocation_status="success"}[5m]))
	//   /
	// sum(rate(buildbuddy_invocation_count[5m]))
	// ```

	InvocationDurationUs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "duration_usec",
		Buckets:   durationUsecBuckets(1*time.Millisecond, 1*day, 10),
		Help:      "The total duration of each invocation, in **microseconds**.",
	}, []string{
		InvocationStatusLabel,
		BazelCommand,
	})

	// #### Examples
	//
	// ```promql
	// # Median invocation duration in the past 5 minutes
	// histogram_quantile(
	//   0.5,
	//   sum(rate(buildbuddy_invocation_duration_usec_bucket[5m])) by (le)
	// )
	// ```

	// InvocationDurationUsExported is a simplified version of
	// InvocationDurationUs which is exported to customers.
	InvocationDurationUsExported = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "duration_usec_exported",
		Buckets:   durationUsecBuckets(1*time.Millisecond, 1*day, 10),
		Help:      "The total duration of each invocation, in **microseconds**.",
	}, []string{
		InvocationStatusLabel,
		GroupID,
	})

	InvocationOpenStreams = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "open_streams",
		Help:      "Number of build event streams currently being handled by the server.",
	})

	BuildEventCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "build_event_count",
		Help:      "Number of [build events](https://docs.bazel.build/versions/master/build-event-protocol.html) uploaded to BuildBuddy.",
	}, []string{
		StatusLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Build events uploaded per second
	// sum(rate(buildbuddy_invocation_build_event_count[5m]))
	//
	// # Approximate error rate of build event upload handler
	// sum(rate(buildbuddy_invocation_build_event_count{status="0"}[5m]))
	//   /
	// sum(rate(buildbuddy_invocation_build_event_count[5m]))
	// ```

	StatsRecorderWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "stats_recorder_workers",
		Help:      "Number of invocation stats recorder workers currently running.",
	})

	StatsRecorderDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "stats_recorder_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "How long it took to finalize an invocation's stats, in **microseconds**. This includes the time required to wait for all BuildBuddy apps to flush their local metrics to Redis (if applicable) and then record the metrics to the DB.",
	})

	WebhookInvocationLookupWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "webhook_invocation_lookup_workers",
		Help:      "Number of webhook invocation lookup workers currently running.",
	})

	WebhookInvocationLookupDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "webhook_invocation_lookup_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "How long it took to lookup an invocation before posting to the webhook, in **microseconds**.",
	})

	WebhookNotifyWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "webhook_notify_workers",
		Help:      "Number of webhook notify workers currently running.",
	})

	WebhookNotifyDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "invocation",
		Name:      "webhook_notify_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "How long it took to post an invocation proto to the webhook, in **microseconds**.",
	})

	// ## Remote cache metrics
	//
	// NOTE: Cache metrics are recorded at the end of each invocation,
	// which means that these metrics provide _approximate_ real-time signals.

	CacheEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "events",
		Help:      "Number of cache events handled.",
	}, []string{
		CacheTypeLabel,
		CacheEventTypeLabel,
		GroupID,
	})

	CacheNumHitsExported = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "num_hits_exported",
		Help:      "Number of cache hits.",
	}, []string{
		CacheTypeLabel,
		GroupID,
	})

	CacheDownloadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "download_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes downloaded from the remote cache in each download. Use the **`_sum`** suffix to get the total downloaded bytes and the **`_count`** suffix to get the number of downloaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
	})

	CacheDownloadSizeBytesExported = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "download_size_bytes_exported",
		Help:      "Number of bytes downloaded from the remote cache.",
	}, []string{
		GroupID,
		CacheRequestOrigin,
	})
	// #### Examples
	//
	// ```promql
	// # Cache download rate (bytes per second)
	// sum(rate(buildbuddy_cache_download_size_bytes_sum[5m]))
	// ```

	CacheDownloadDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "download_duration_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*time.Hour, 5),
		Help:      "Download duration for each file downloaded from the remote cache, in **microseconds**.",
	}, []string{
		CacheTypeLabel,
	})

	CacheRequestedInlineSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "requested_inline_size_bytes",
		Buckets:   exponentialBucketRange(1, 64*1024*1024, 8),
		Help:      "Size in bytes of the first file requested to be inlined (if any).",
	}, []string{})

	// #### Examples
	//
	// ```promql
	// # Median download duration for content-addressable store (CAS)
	// histogram_quantile(
	//   0.5,
	//   sum(rate(buildbuddy_remote_cache_download_duration_usec{cache_type="cas"}[5m])) by (le)
	// )
	// ```

	CacheUploadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "upload_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes uploaded to the remote cache in each upload. Use the **`_sum`** suffix to get the total uploaded bytes and the **`_count`** suffix to get the number of uploaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
	})

	CacheUploadSizeBytesExported = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "upload_size_bytes_exported",
		Help:      "Number of bytes uploaded to the remote cache",
	}, []string{
		GroupID,
		CacheRequestOrigin,
	})

	// #### Examples
	//
	// ```promql
	// # Cache upload rate (bytes per second)
	// sum(rate(buildbuddy_cache_upload_size_bytes_sum[5m]))
	// ```

	CacheUploadDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "upload_duration_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*time.Hour, 5),
		Help:      "Upload duration for each file uploaded to the remote cache, in **microseconds**.",
	}, []string{
		CacheTypeLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Median upload duration for content-addressable store (CAS)
	// histogram_quantile(
	//   0.5,
	//   sum(rate(buildbuddy_remote_cache_upload_duration_usec{cache_type="cas"}[5m])) by (le)
	// )
	// ```

	DiskCacheLastEvictionAgeUsec = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_last_eviction_age_usec",
		Help:      "The age of the item most recently evicted from the cache, in **microseconds**.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	// This metric is in milliseconds because Grafana heatmaps don't display
	// microsecond durations nicely when they can contain large durations.
	DiskCacheEvictionAgeMsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_eviction_age_msec",
		Buckets: customDurationBuckets(
			time.Duration.Milliseconds,
			[]time.Duration{
				6 * time.Hour,
				12 * time.Hour,
				1 * day,
				2 * day,
				3 * day,
				4 * day,
				5 * day,
				6 * day,
				7 * day,
				14 * day,
				21 * day,
			}),
		Help: "Age of items evicted from the cache, in **milliseconds**.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	DiskCacheNumEvictions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_num_evictions",
		Help:      "Number of items evicted.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	DiskCacheBytesEvicted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_partition_size_bytes_evicted",
		Help:      "Number of bytes in the partition evicted.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	DiskCachePartitionSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_partition_size_bytes",
		Help:      "Number of bytes in the partition.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	DiskCacheSampledPartitionGroupSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_sampled_partition_group_size_bytes",
		Help:      "Number of bytes seen while sampling the partition for eviction, by group ID.",
	}, []string{
		PartitionID,
		CacheNameLabel,
		GroupID,
	})

	DiskCachePartitionCapacityBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_partition_capacity_bytes",
		Help:      "Number of bytes in the partition.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	DiskCachePartitionNumItems = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_partition_num_items",
		Help:      "Number of items in the partition.",
	}, []string{
		PartitionID,
		CacheNameLabel,
		CacheTypeLabel,
	})

	DiskCacheDuplicateWrites = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_duplicate_writes",
		Help:      "Number of writes for digests that already exist.",
	}, []string{
		CacheNameLabel,
	})

	DiskCacheAddedFileSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_added_file_size_bytes",
		Help:      "Size of artifacts added to the file cache, in **bytes**.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 40),
	}, []string{
		CacheNameLabel,
	})

	DiskCacheFilesystemTotalBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_filesystem_total_bytes",
		Help:      "Total size of the underlying filesystem.",
	}, []string{
		CacheNameLabel,
	})

	DiskCacheFilesystemAvailBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_filesystem_avail_bytes",
		Help:      "Available bytes in the underlying filesystem.",
	}, []string{
		CacheNameLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Total number of duplicate writes.
	// sum(buildbuddy_remote_cache_duplicate_writes)
	// ```

	DiskCacheDuplicateWritesBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "disk_cache_duplicate_writes_bytes",
		Help:      "Number of bytes written that already existed in the cache.",
	}, []string{
		CacheNameLabel,
	})

	DistributedCachePeerLookups = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "distributed_cache_peer_lookups",
		Help:      "Number of peers consulted (including the 'local peer') for a distributed cache read before returning a response. For batch requests, one observation is recorded for each digest in the request.",
		Buckets:   prometheus.LinearBuckets(0, 1, 10),
	}, []string{
		DistributedCacheOperation,
		CacheHitMissStatus,
	})

	DistributedCacheBackfills = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "distributed_cache_backfills",
		Help:      "Number of backfilled digests. ",
	}, []string{
		GroupID,
		StatusLabel,
	})

	MigrationNotFoundErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "migration_not_found_error_count",
		Help:      "Number of not found errors from the destination cache during a cache migration.",
	}, []string{
		CacheRequestType,
		GroupID,
	})

	MigrationDoubleReadHitCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "migration_double_read_hit_count",
		Help:      "Number of double reads where the source and destination caches hold the same digests during a cache migration.",
	}, []string{
		CacheRequestType,
		GroupID,
	})

	MigrationCopyChanSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "migration_copy_chan_size",
		Help:      "Number of digests queued to be copied during a cache migration.",
	})

	MigrationBytesCopied = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "migration_bytes_copied",
		Help:      "Number of bytes copied from the source to destination cache during a cache migration.",
	}, []string{
		CacheTypeLabel,
		GroupID,
	})

	MigrationBlobsCopied = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "migration_blobs_copied",
		Help:      "Number of blobs copied from the source to destination cache during a cache migration.",
	}, []string{
		CacheTypeLabel,
		GroupID,
	})

	TreeCacheLookupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "tree_cache_lookup_count",
		Help:      "Total number of TreeCache lookups.",
	}, []string{
		TreeCacheLookupStatus,
		TreeCacheLookupLevel,
	})

	TreeCacheSplitLookupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "tree_cache_split_lookup_count",
		Help:      "Total number of TreeCache split lookups.",
	}, []string{
		TreeCacheSplitLookupStatus,
	})

	TreeCacheSplitWriteCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "tree_cache_split_write_count",
		Help:      "Total number of splits written to TreeCache.",
	})

	TreeCacheSetCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "tree_cache_set_count",
		Help:      "Total number of TreeCache sets.",
	}, []string{
		TreeCacheSetStatus,
	})

	TreeCacheBytesTransferred = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "tree_cache_bytes_transferred",
		Help:      "Number of bytes written or read from tree cache",
	}, []string{
		TreeCacheOperation,
	})

	GetTreeDirectoryLookupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "get_tree_directory_lookup_count",
		Help:      "Number of directories fetched by GetTree calls, split by where the directory was found.",
	}, []string{
		GetTreeLookupLocation,
	})

	GetTreeFilecacheTreesRead = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "get_tree_filecache_trees_read",
		Help:      "Number of trees read from the local filecache.",
	})

	GetTreeFilecacheBytesRead = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "get_tree_filecache_bytes_read",
		Help:      "Total size in bytes of trees read from the local filecache.",
	})

	GetTreeFilecacheTreesWritten = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "get_tree_filecache_trees_written",
		Help:      "Number of trees written to the local filecache.",
	})

	GetTreeFilecacheBytesWritten = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "get_tree_filecache_bytes_written",
		Help:      "Total size in bytes of trees written to the local filecache.",
	})

	LookasideCacheLookupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "lookaside_cache_lookup_count",
		Help:      "Total number of Lookaside Cache lookups.",
	}, []string{
		LookasideCacheLookupStatus,
	})

	// This metric is in milliseconds because Grafana heatmaps don't display
	// microsecond durations nicely when they can contain large durations.
	LookasideCacheEvictionAgeMsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "lookaside_cache_eviction_age_msec",
		Buckets:   durationMsecBuckets(30*time.Minute, 24*time.Hour, 2),
		Help:      "Age of items evicted from the cache, in **milliseconds**.",
	}, []string{
		LookasideCacheEvictionReason,
	})

	// ## Remote execution metrics

	RemoteExecutionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "count",
		Help:      "Number of actions executed remotely. This only includes actions which reached the execution phase. If an action fails before execution (for example, if it fails authentication) then this metric is not incremented.",
	}, []string{
		ExitCodeLabel,
		StatusHumanReadableLabel,
		IsolationTypeLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Total number of actions executed per second
	// sum(rate(buildbuddy_remote_execution_count[5m]))
	// ```

	RemoteExecutionTasksStartedCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "tasks_started_count",
		Help:      "Number of tasks started remotely, but not necessarily completed. Includes retry attempts of the same task.",
	})

	RemoteExecutionExecutedActionMetadataDurationsUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "executed_action_metadata_durations_usec",
		Buckets:   durationUsecBuckets(1*time.Millisecond, 1*day, 2.0),
		Help:      "Time spent in each stage of action execution, in **microseconds**. Queries should filter or group by the `stage` label, taking care not to aggregate different stages.",
	}, []string{
		ExecutedActionStageLabel,
		GroupID,
	})

	RemoteExecutionDurationUsecExported = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "duration_usec_exported",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*day, 2),
		Help:      "Time spent in remote execution, in **microseconds**. ",
	}, []string{
		OS,
		GroupID,
	})

	// #### Examples
	//
	// ```promql
	// # Median duration of all command stages
	// histogram_quantile(
	//	  0.5,
	//   sum(rate(buildbuddy_remote_execution_executed_action_metadata_durations_usec_bucket[5m])) by (le, stage)
	// )
	//
	// # p90 duration of just the command execution stage
	// histogram_quantile(
	//	  0.9,
	//   sum(rate(buildbuddy_remote_execution_executed_action_metadata_durations_usec_bucket{stage="execution"}[5m])) by (le)
	// )
	// ```

	RemoteExecutionTaskPressureStallDurationFraction = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "task_pressure_stall_duration_fraction",
		Help:      "Linux PSI stall time as a fraction of each action's execution duration (0-1).",
		Buckets: []float64{
			0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.07, 0.08, 0.09, 0.1,
			0.2, 0.3, 0.4, 0.5, 0.75, 1,
		},
	}, []string{
		PSIResourceLabel,
		PSIStallTypeLabel,
	})

	RemoteExecutionTaskSizeReadRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "task_size_read_requests",
		Help:      "Number of read requests to the task sizer, which estimates action resource usage based on historical execution stats.",
	}, []string{
		TaskSizeReadStatusLabel,
		IsolationTypeLabel,
		OS,
		Arch,
		GroupID,
	})

	RemoteExecutionTaskSizeWriteRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "task_size_write_requests",
		Help:      "Number of write requests to the task sizer, which estimates action resource usage based on historical execution stats.",
	}, []string{
		TaskSizeWriteStatusLabel,
		IsolationTypeLabel,
		OS,
		Arch,
		GroupID,
	})

	RemoteExecutionTaskSizePredictionDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "task_size_prediction_duration_usec",
		Help:      "Task size prediction model request duration in **microseconds**.",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*time.Second, 2),
	}, []string{
		StatusHumanReadableLabel,
	})

	RemoteExecutionEnqueuedTaskMilliCPU = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "enqueued_task_milli_cpu",
		Help:      "Milli-CPU prediction of enqueued tasks.",
		Buckets:   exponentialBucketRange(1, 128_000, 1.2),
	})

	RemoteExecutionEnqueuedTaskMemoryBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "enqueued_task_memory_bytes",
		Help:      "Memory prediction of enqueued tasks.",
		Buckets:   exponentialBucketRange(1, 1024*1024*1024*1024 /*1 TB*/, 1.5),
	})

	RemoteExecutionWaitingExecutionResult = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "waiting_execution_result",
		Help:      "Number of execution requests for which the client is actively waiting for results.",
	}, []string{
		GroupID,
	})

	// #### Examples
	//
	// ```promql
	// # Total number of execution requests with client waiting for result.
	// sum(buildbuddy_remote_execution_waiting_execution_result)
	// ```

	RemoteExecutionRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "requests",
		Help:      "Number of execution requests received.",
	}, []string{
		GroupID,
		OS,
		Arch,
	})

	RemoteExecutionExecutorRegistrationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "executor_registration_count",
		Help:      "Number of executor registrations on the scheduler.",
	}, []string{
		VersionLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Rate of new execution requests by OS/Arch.
	// sum(rate(buildbuddy_remote_execution_requests[1m])) by (os, arch)
	// ```

	RemoteExecutionMergedActions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "merged_actions",
		Help:      "Number of identical execution requests that have been merged.",
	}, []string{
		GroupID,
	})

	RemoteExecutionHedgedActions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "hedged_actions",
		Help:      "Number of identicial execution request which were merged for which a hedged execution was run in the background.",
	}, []string{
		GroupID,
	})

	RemoteExecutionMergedActionsPerExecution = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "merged_actions_per_execution",
		Buckets:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 20, 25, 30, 35, 40, 45, 50, 60, 70, 80, 90, 100},
		Help:      "Distribution of how many actions were submitted and merged against a single, canonical execution over the lifetime of that canonical execution. Note that this metric is recorded once per merged-action, so distribution values are cumulative, or recorded n-times per canonical execution.",
	}, []string{
		GroupID,
	})

	RemoteExecutionMergedActionSubmitTimeOffsetUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "merged_action_submit_time_offset_usec",
		Buckets:   durationUsecBuckets(1*time.Millisecond, 1*day, 10),
		Help:      "The offset, in microseconds of wall-time, between the time when a merged action was submitted to the execution server and when the original action was submitted to the execution server.",
	}, []string{
		GroupID,
	})

	// #### Examples
	//
	// ```promql
	// # Rate of merged actions by group.
	// sum(rate(buildbuddy_remote_execution_merged_actions[1m])) by (group_id)
	// ```

	// Note: RemoteExecutionQueueLength is exported to customers.
	RemoteExecutionQueueLength = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "queue_length",
		Help:      "Number of actions currently waiting in the executor queue.",
	}, []string{
		GroupID,
	})

	// #### Examples
	//
	// ```promql
	// # Median queue length across all executors
	// quantile(0.5, buildbuddy_remote_execution_queue_length)
	// ```

	RemoteExecutionTasksExecuting = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "tasks_executing",
		Help:      "Number of tasks currently being executed by the executor.",
	}, []string{
		ExecutedActionStageLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Fraction of idle executors
	// count_values(0, buildbuddy_remote_execution_tasks_executing)
	//   /
	// count(buildbuddy_remote_execution_tasks_executing)
	// ```

	RemoteExecutionAssignedRAMBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assigned_ram_bytes",
		Help:      "Estimated RAM on the executor that is currently allocated for task execution, in **bytes**.",
	})

	RemoteExecutionAssignedOrQueuedEstimatedRAMBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assigned_and_queued_estimated_ram_bytes",
		Help:      "Estimated RAM on the executor that is currently allocated for queued or executing tasks, in **bytes**. Note that this is a fuzzy estimate because there's no guarantee that tasks queued on a machine will be handled by that machine.",
	})

	RemoteExecutionAssignableRAMBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assignable_ram_bytes",
		Help:      "Maximum total RAM that can be allocated for task execution, in **bytes**.",
	})

	RemoteExecutionAssignedMilliCPU = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assigned_milli_cpu",
		Help:      "Estimated CPU time on the executor that is currently allocated for task execution, in **milliCPU** (CPU-milliseconds per second).",
	})

	RemoteExecutionAssignedOrQueuedEstimatedMilliCPU = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assigned_and_queued_estimated_milli_cpu",
		Help:      "Estimated CPU time on the executor that is currently allocated for queued or executing tasks, in **milliCPU** (CPU-milliseconds per second). Note that this is a fuzzy estimate because there's no guarantee that tasks queued on a machine will be handled by that machine.",
	})

	RemoteExecutionAssignableMilliCPU = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "assignable_milli_cpu",
		Help:      "Maximum total CPU time on the executor that can be allocated for task execution, in **milliCPU** (CPU-milliseconds per second).",
	})

	FileDownloadCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_download_count",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of files downloaded during remote execution.",
	})

	FileDownloadSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_download_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Total number of bytes downloaded during remote execution.",
	})

	FileDownloadDurationUsec = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_download_duration_usec",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Per-file download duration during remote execution, in **microseconds**.",
	})

	FileUploadCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_upload_count",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of files uploaded during remote execution.",
	})

	FileUploadSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_upload_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Total number of bytes uploaded during remote execution.",
	})

	SkippedOutputBytes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "skipped_output_bytes",
		Help:      "Total number of output bytes that weren't uploaded after remote execution.",
	})

	FileUploadDurationUsec = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_upload_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Per-file upload duration during remote execution, in **microseconds**.",
	})

	NetworkingCommandDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "networking_command_duration_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 10*time.Minute, 1.5),
		Help:      "Duration of networking commands, in **microseconds**.",
	}, []string{
		CommandName,
	})

	VFSCASFilesCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "vfs_cas_files_count",
		Help:      "Total number of CAS files in VFS filesystems.",
	})

	VFSCASFilesAccessedCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "vfs_cas_files_accessed_count",
		Help:      "Number of CAS files in VFS filesystems that were accessed by the action.",
	})

	VFSCASFilesSizeBytes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "vfs_cas_files_size_bytes",
		Help:      "Total size of CAS files in VFS filesystems.",
	})

	VFSCASFilesAccessedBytes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "vfs_cas_files_accessed_bytes",
		Help:      "Size of CAS files in VFS filesystems that were accessed by the action.",
	})

	NetworkingCommandCPUUsageUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "networking_command_cpu_usage_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 10*time.Minute, 1.5),
		Help:      "CPU usage of networking commands, in **CPU-microseconds**.",
	}, []string{
		CommandName,
	})

	FirecrackerStageDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "stage_duration_usec",
		Buckets: customDurationBuckets(
			time.Duration.Microseconds,
			[]time.Duration{
				25 * time.Millisecond,
				50 * time.Millisecond,
				100 * time.Millisecond,
				500 * time.Millisecond,
				750 * time.Millisecond,
				1 * time.Second,
				3 * time.Second,
				5 * time.Second,
				10 * time.Second,
				15 * time.Second,
				20 * time.Second,
				30 * time.Second,
				45 * time.Second,
				1 * time.Minute,
				90 * time.Second,
				2 * time.Minute,
				3 * time.Minute,
				5 * time.Minute,
				8 * time.Minute,
				15 * time.Minute,
				30 * time.Minute,
				45 * time.Minute,
				1 * time.Hour,
				3 * time.Hour,
				5 * time.Hour,
				8 * time.Hour,
				15 * time.Hour,
				24 * time.Hour,
			}),
		Help: "The total duration of each firecracker stage, in microseconds. " +
			"NOTE: Remember that these durations represent the upper bounds of " +
			"histogram buckets. Data points fall within pre-defined buckets," +
			" and don't represent the actual duration of each event.",
	}, []string{
		Stage,
	})

	// #### Stage label values
	// * "init": Time for the VM to start up (either a new VM or from a snapshot)
	// * "exec": Time to run the command inside the container
	// * "task_lifecycle": Time from when the task if first assigned to the VM
	// (beginning of init) to after it's finished execution. This roughly
	// represents what a customer will wait for the task to complete after it's
	// been scheduled to a firecracker runner
	// * "pause": Time to pause the VM, save a snapshot, and cleanup resources
	//
	// #### Examples
	// ```promql
	// # P95 workflow lifecycle duration in the past 5 minutes, grouped by group_id
	// histogram_quantile(
	//   0.95,
	//   sum by(le, group_id) (
	//      rate(buildbuddy_firecracker_stage_duration_usec_bucket{job="executor-workflows", stage="task_lifecycle"}[5m])
	//     )
	//  )
	// ```

	FirecrackerExecDialDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "exec_dial_duration_usec",
		Buckets:   durationUsecBuckets(50*time.Millisecond, 2*time.Minute, 2),
		Help:      "Time taken to dial the VM guest execution server after it has been started or resumed, in **microseconds**.",
	}, []string{
		CreatedFromSnapshot,
	})

	SnapshotRemoteCacheUploadSizeBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "snapshot_remote_cache_upload_size_bytes",
		Help:      "After a copy-on-write snapshot has been used, the total count of compressed bytes written to the cache (i.e. will be 0 if the artifact is already cached).",
	}, []string{
		FileName,
	})

	COWSnapshotDirtyChunkRatio = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_dirty_chunk_ratio",
		Buckets:   prometheus.LinearBuckets(0, .05, 20),
		Help:      "After a copy-on-write snapshot has been used, the ratio of dirty/total chunks.",
	}, []string{
		FileName,
	})

	// #### Examples
	//
	// ```promql
	// # To view how many elements fall into each bucket
	// # Visualize with the Bar Gauge type
	// # Legend: {{le}}
	// # Format: Heatmap
	// sum(increase(buildbuddy_firecracker_cow_snapshot_dirty_chunk_ratio_bucket[5m])) by(le)
	// ```

	COWSnapshotDirtyBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_dirty_bytes",
		Help:      "After a copy-on-write snapshot has been used, the total count of bytes dirtied.",
	}, []string{
		FileName,
	})

	COWSnapshotEmptyChunkRatio = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_empty_chunk_ratio",
		Buckets:   prometheus.LinearBuckets(0, .05, 20),
		Help:      "After a copy-on-write snapshot has been used, the ratio of empty (i.e. all 0s) /total chunks.",
	}, []string{
		FileName,
	})

	COWSnapshotChunkSourceRatio = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_chunk_source_ratio",
		Buckets:   prometheus.LinearBuckets(0, .1, 10),
		Help:      "After a copy-on-write snapshot has been used, the percentage of chunks that were initialized by the given source.",
	}, []string{
		FileName,
		ChunkSource,
	})

	// TODO(Maggie): Delete after evaluating the results
	COWSnapshotSkippedRemoteBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_skipped_remote_bytes",
		Help:      "The number of uncompressed bytes that were not written to the remote cache due to only writing locally.",
	}, []string{
		FileName,
	})

	COWSnapshotMemoryMappedBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_memory_mapped_bytes",
		Help:      "Total number of bytes currently memory-mapped.",
	}, []string{
		FileName,
	})

	COWSnapshotPageFaultTotalDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_page_fault_total_duration_usec",
		Buckets:   durationUsecBuckets(100*time.Millisecond, 10*time.Minute, 2.3),
		Help:      "For a snapshotted VM, total time spent fulfilling page faults.",
	}, []string{
		Stage,
	})

	COWSnapshotChunkOperationTotalDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_snapshot_chunk_operation_duration_usec",
		Buckets:   durationUsecBuckets(100*time.Millisecond, 10*time.Minute, 2.3),
		Help:      "For a COW snapshot, cumulative time spent on an operation type.",
	}, []string{
		FileName,
		EventName,
		Stage,
	})

	COWBytesRead = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_bytes_read",
		Help:      "Total number of bytes read from COW chunked files.",
	}, []string{
		FileName,
	})

	COWBytesWritten = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "cow_bytes_written",
		Help:      "Total number of bytes written to COW chunked files.",
	}, []string{
		FileName,
	})

	FirecrackerWorkspaceDiskWriteOps = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "firecracker",
		Name:      "workspace_conversion_disk_write_ops",
		Help:      "Total number of disk write operations performed converting Firecracker action workspaces to/from ext4 images.",
	}, []string{
		CommandName,
	})

	MaxRecyclableResourceUsageEvent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "max_recyclable_resource_usage_event",
		Help:      "Counter for firecracker runners that reach max disk/memory usage and won't get recycled.",
	}, []string{
		GroupID,
		EventName,
		RecycledRunnerStatus,
	})

	RecycleRunnerRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "recycle_runner_requests",
		Help:      "Number of execution requests with runner recycling enabled (via the platform property `recycle-runner=true`).",
	}, []string{
		RecycleRunnerRequestStatusLabel,
	})

	RunnerPoolCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "runner_pool_count",
		Help:      "Number of command runners that are currently pooled (and available for recycling).",
	})

	RunnerPoolEvictions = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "runner_pool_evictions",
		Help:      "Number of command runners removed from the pool to make room for other runners.",
	})

	RunnerPoolFailedRecycleAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "runner_pool_failed_recycle_attempts",
		Help:      "Number of failed attempts to add runners to the pool.",
	}, []string{
		RunnerPoolFailedRecycleReason,
	})

	RunnerPoolMemoryUsageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "runner_pool_memory_usage_bytes",
		Help:      "Total memory usage of pooled command runners, in **bytes**. Currently only supported for Docker-based executors.",
	})

	RunnerPoolDiskUsageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "runner_pool_disk_usage_bytes",
		Help:      "Total disk usage of pooled command runners, in **bytes**.",
	})

	FileCacheRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_cache_requests",
		Help:      "Number of local executor file cache requests.",
	}, []string{
		FileCacheRequestStatusLabel,
	})

	FileCacheOpLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_cache_op_latency_usec",
		Help:      "Latency of individual file cache operations.",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*time.Second, 10),
	}, []string{
		OpLabel,
	})

	FileCacheLastEvictionAgeUsec = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_cache_last_eviction_age_usec",
		Help:      "Age of the last entry evicted from the executor's local file cache (relative to when it was added to the cache), in **microseconds**.",
	})

	FileCacheAddedFileSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_cache_added_file_size_bytes",
		Help:      "Size of artifacts added to the file cache, in **bytes**.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 40),
	})

	FileCacheAddedFileBytesCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "file_cache_added_file_bytes_count",
		Help:      "Total number of bytes written to the filecache by groupid.",
	}, []string{
		GroupID,
	})

	// ## Blobstore metrics
	//
	// "Blobstore" refers to the backing storage that BuildBuddy uses to
	// store objects in the cache, as well as certain pieces of temporary
	// data (such as invocation events while an invocation is in progress).

	BlobstoreReadCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "read_count",
		Help:      "Number of files read from the blobstore.",
	}, []string{
		StatusLabel,
		BlobstoreTypeLabel,
	})

	BlobstoreReadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "read_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes read from the blobstore per file.",
	}, []string{
		BlobstoreTypeLabel,
	})

	// ```promql
	// # Bytes downloaded per second
	// sum(rate(buildbuddy_blobstore_read_size_bytes[5m]))
	// ```

	BlobstoreReadDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "read_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Duration per blobstore file read, in **microseconds**.",
	}, []string{
		BlobstoreTypeLabel,
	})

	BlobstoreWriteCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "write_count",
		Help:      "Number of files written to the blobstore.",
	}, []string{
		StatusLabel,
		BlobstoreTypeLabel,
	})

	// ```promql
	// # Bytes uploaded per second
	// sum(rate(buildbuddy_blobstore_write_size_bytes[5m]))
	// ```

	BlobstoreWriteSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "write_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes written to the blobstore per file.",
	}, []string{
		BlobstoreTypeLabel,
	})

	BlobstoreWriteDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "write_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Duration per blobstore file write, in **microseconds**.",
	}, []string{
		BlobstoreTypeLabel,
	})

	BlobstoreDeleteCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "delete_count",
		Help:      "Number of files deleted from the blobstore.",
	}, []string{
		StatusLabel,
		BlobstoreTypeLabel,
	})

	BlobstoreDeleteDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "delete_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Delete duration per blobstore file deletion, in **microseconds**.",
	}, []string{
		BlobstoreTypeLabel,
	})

	BlobstoreExistsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "exists_count",
		Help:      "Number of existance checks in the blobstore.",
	}, []string{
		StatusLabel,
		BlobstoreTypeLabel,
	})

	BlobstoreExistsDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "blobstore",
		Name:      "exists_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "CheckExists duration, in **microseconds**.",
	}, []string{
		BlobstoreTypeLabel,
	})

	// # SQL metrics
	//
	// The following metrics are for monitoring the SQL database configured
	// for BuildBuddy.
	//
	// If you'd like to see an up-to-date catalog of what BuildBuddy stores in
	// its SQL database, see the table definitions [here](https://github.com/buildbuddy-io/buildbuddy/blob/master/server/tables/tables.go).
	//
	// ## Query / error rate metrics

	SQLQueryCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "query_count",
		Help:      "Number of SQL queries executed.",
	}, []string{
		SQLQueryTemplateLabel,
	})

	// #### Examples
	//
	// ```promql
	// # SQL queries per second (by query template).
	// sum by (sql_query_template) (rate(buildbuddy_sql_query_count[5m]))
	// ```

	SQLQueryDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "query_duration_usec",
		Buckets:   durationUsecBuckets(1*time.Millisecond, 1*time.Hour, 1.2),
		Help:      "SQL query duration, in **microseconds**.",
	}, []string{
		SQLQueryTemplateLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Median SQL query duration
	// histogram_quantile(
	//	  0.5,
	//   sum(rate(buildbuddy_sql_query_duration_usec_bucket[5m])) by (le)
	// )
	// ```

	SQLErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "error_count",
		Help:      "Number of SQL queries that resulted in an error.",
	}, []string{
		SQLQueryTemplateLabel,
	})

	// #### Examples
	//
	// ```promql
	// # SQL error rate
	// sum(rate(buildbuddy_sql_error_count[5m]))
	//   /
	// sum(rate(buildbuddy_sql_query_count[5m]))
	// ```

	// ## `database/sql` metrics
	//
	// The following metrics directly expose
	// [DBStats](https://golang.org/pkg/database/sql/#DBStats) from the
	// `database/sql` Go package.

	SQLMaxOpenConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "max_open_connections",
		Help:      "Maximum number of open connections to the database.",
	}, []string{
		SQLDBRoleLabel,
	})

	SQLOpenConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "open_connections",
		Help:      "The number of established connections to the database.",
	}, []string{
		SQLConnectionStatusLabel,
		SQLDBRoleLabel,
	})

	SQLWaitCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "wait_count",
		Help:      "The total number of connections waited for.",
	}, []string{
		SQLDBRoleLabel,
	})

	SQLWaitDuration = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "wait_duration_usec",
		Help:      "The total time blocked waiting for a new connection, in **microseconds**.",
	}, []string{
		SQLDBRoleLabel,
	})

	SQLMaxIdleClosed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "max_idle_closed",
		Help:      "The total number of connections closed due to SetMaxIdleConns.",
	}, []string{
		SQLDBRoleLabel,
	})

	SQLMaxIdleTimeClosed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "max_idle_time_closed",
		Help:      "The total number of connections closed due to SetConnMaxIdleTime.",
	}, []string{
		SQLDBRoleLabel,
	})

	SQLMaxLifetimeClosed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "sql",
		Name:      "max_lifetime_closed",
		Help:      "The total number of connections closed due to SetConnMaxLifetime.",
	}, []string{
		SQLDBRoleLabel,
	})

	// ## HTTP metrics

	HTTPRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "http",
		Name:      "request_count",
		Help:      "HTTP request count.",
	}, []string{
		HTTPRouteLabel,
		HTTPMethodLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Requests per second, by status code
	// sum by (code) (rate(buildbuddy_http_request_count[5m]))
	//
	// # 5xx error ratio
	// sum(rate(buildbuddy_http_request_count{code=~"5.."}[5m]))
	//   /
	// sum(rate(buildbuddy_http_request_count[5m]))
	// ```

	HTTPRequestHandlerDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "http",
		Name:      "request_handler_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Time taken to handle each HTTP request in **microseconds**.",
	}, []string{
		HTTPRouteLabel,
		HTTPMethodLabel,
		HTTPResponseCodeLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Median request duration for successfuly processed (2xx) requests.
	// # Other status codes may be associated with early-exits and are
	// # likely to add too much noise.
	// histogram_quantile(
	//   0.5,
	//   sum by (le)	(rate(buildbuddy_http_request_handler_duration_usec{code=~"2.."}[5m]))
	// )
	// ```

	HTTPResponseSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "http",
		Name:      "response_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Response size of each HTTP response in **bytes**.",
	}, []string{
		HTTPRouteLabel,
		HTTPMethodLabel,
		HTTPResponseCodeLabel,
	})

	// #### Examples
	//
	// ```promql
	// # Median HTTP response size
	// histogram_quantile(
	//   0.5,
	//   sum by (le)	(rate(buildbuddy_http_response_size_bytes[5m]))
	// )
	// ```

	HTTPClientRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "http",
		Name:      "client_request_count",
		Help:      "HTTP outgoing request count.",
	}, []string{
		HTTPHostLabel,
		HTTPMethodLabel,
	})

	HTTPClientResponseSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "http",
		Name:      "client_response_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Response size of response for each HTTP client request in **bytes**.",
	}, []string{
		HTTPHostLabel,
		HTTPMethodLabel,
		HTTPResponseCodeLabel,
	})

	// ## Internal metrics
	//
	// These metrics are for monitoring lower-level subsystems of BuildBuddy.
	//
	// ### Build event handler
	//
	// The build event handler logs all build events uploaded to BuildBuddy
	// as part of the Build Event Protocol.

	BuildEventHandlerDurationUs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "build_event_handler",
		Name:      "duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent handling each build event in **microseconds**.",
	}, []string{
		StatusLabel,
	})

	// ### Webhooks
	//
	// Webhooks are HTTP endpoints exposed by BuildBuddy server which allow it to
	// respond to repository events. These URLs are created as part of BuildBuddy
	// workflows.

	WebhookHandlerWorkflowsStarted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "webhook_handler",
		Name:      "workflows_started",
		Help:      "The number of workflows triggered by the webhook handler.",
	}, []string{
		WebhookEventName,
	})

	// ### Cache
	//
	// "Cache" refers to the cache backend(s) that BuildBuddy uses to
	// accelerate file IO operations, which are common in different
	// subsystems such as the remote cache and the fetch server (for
	// downloading invocation artifacts).
	//
	// BuildBuddy can be configured to use multiple layers of caching
	// (an in-memory layer, coupled with a cloud storage layer).
	//
	// #### `get` metrics
	//
	// `get` metrics track non-streamed cache reads (all data is fetched
	// from the cache in a single request).

	CacheGetCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "get_count",
		Help:      "Number of cache get requests.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheGetDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "get_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent retrieving each entry from the cache, in **microseconds**. This is recorded only for successful gets.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheGetSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "get_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Size of each entry retrieved from the cache, in **bytes**. This is recorded only for successful gets.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	// #### `read` metrics
	//
	// `read` metrics track streamed cache reads.

	CacheReadCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "read_count",
		Help:      "Number of streamed cache reads started. This is incremented once for each started stream, **not** for each chunk in the stream.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheReadDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "read_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The total time spent for each read stream, in **microseconds**. This is recorded only for successful reads, and measures the entire read stream (not just individual chunks).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheReadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "read_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Total size of each entry retrieved from the cache via streaming, in **bytes**. This is recorded only on success, and measures the entire stream (not just individual chunks).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	// #### `set` metrics
	//
	// `set` metrics track non-streamed cache writes (all data is wrtiten
	// in a single request).

	CacheSetCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "set_count",
		Help:      "Number of cache set requests.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheSetDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "set_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent writing each entry to the cache, in **microseconds**. This is recorded only for successful sets.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheSetSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "set_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Size of the value stored in each set operation, in **bytes**. This is recorded only for successful sets.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheSetRetryCount = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "set_retries",
		Buckets:   prometheus.LinearBuckets(0, 1, 10),
		Help:      "Number of retries required to fulfill the set request (an observed value of 0 means the transfer succeeded on the first try).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	// #### `write` metrics
	//
	// `write` metrics track streamed cache writes.

	CacheWriteCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "write_count",
		Help:      "Number of streamed cache writes started. This is incremented once for each started stream, **not** for each chunk in the stream.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheWriteDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "write_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent for each streamed write to the cache, in **microseconds**. This is recorded only on success, and measures the entire stream (not just individual chunks).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheWriteSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "write_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Size of each entry written to the cache via streaming, in **bytes**. This is recorded only on success, and measures the entire stream (not just individual chunks).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheWriteRetryCount = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "write_retries",
		Buckets:   prometheus.LinearBuckets(0, 1, 10),
		Help:      "Number of retries required to write each chunk in the stream (an observed value of 0 means the transfer succeeded on the first try).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	// ### Other cache metrics

	CacheDeleteCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "delete_count",
		Help:      "Number of deletes from the cache.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheDeleteDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "delete_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Duration of each cache deletion, in **microseconds**.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheContainsCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "contains_count",
		Help:      "Number of `contains(key)` requests made to the cache.",
	}, []string{
		StatusLabel,
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheContainsDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "contains_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "Duration of each each `contains(key)` request, in **microseconds**.",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	CacheContainsRetryCount = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "cache",
		Name:      "contains_retry_count",
		Buckets:   prometheus.LinearBuckets(0, 1, 10),
		Help:      "Number of retries required to fulfill each `contains(key)` request to the cache (an observed value of 0 means the request succeeded on the first try).",
	}, []string{
		CacheTierLabel,
		CacheBackendLabel,
	})

	// ### Misc metrics

	Version = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Name:      "version",
		Help:      "Binary version of the running instance. Always reports a value of 1 similar to the `up` metric, but has a label containing the version.",
	}, []string{
		VersionLabel,
		CommitLabel,
	})

	UnexpectedEvent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Name:      "unexpected_event",
		Help:      "Counter for unexpected events.",
	}, []string{
		EventName,
	})

	HealthCheck = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "health_check",
		Name:      "status",
		Help:      "Health check status.",
	}, []string{
		HealthCheckName,
	})

	QuotaExceeded = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "quota",
		Name:      "quota_exceeded_count",
		Help:      "Total number of calls banned by quota server",
	}, []string{
		QuotaNamespace,
		QuotaKey,
	})

	QuotaKeyEmptyCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "quota",
		Name:      "quota_key_empty_count",
		Help:      "Total number of calls with empty quota key",
	}, []string{
		QuotaNamespace,
	})

	ClickhouseInsertedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "clickhouse",
		Name:      "insert_count",
		Help:      "Num of rows inserted into ClickHouse",
	}, []string{
		ClickhouseTableName,
		ClickhouseStatusLabel,
	})
	ClickhouseQueryDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "clickhouse",
		Name:      "query_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "ClickHouse query duration, in **microseconds**.",
	}, []string{
		SQLQueryTemplateLabel,
	})
	ClickhouseQueryErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "clickhouse",
		Name:      "error_count",
		Help:      "Number of ClickHouse SQL queries that resulted in an error.",
	}, []string{
		SQLQueryTemplateLabel,
	})
	ClickhouseQueryCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "clickhouse",
		Name:      "query_count",
		Help:      "Number of ClickHouse SQL queries executed.",
	}, []string{
		SQLQueryTemplateLabel,
	})

	BytesCompressed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "compressor",
		Name:      "bytes_compressed",
		Help:      "The number of decompressed bytes passed into compressors",
	}, []string{
		CompressionType,
	})

	BytesDecompressed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "compressor",
		Name:      "bytes_decompressed",
		Help:      "The number of decompressed bytes passed out of decompressors",
	}, []string{
		CompressionType,
	})

	CompressionRatio = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "pebble",
		Name:      "compression_ratio",
		Buckets:   prometheus.LinearBuckets(0, .05, 40),
		Help: "The aggregate compression ratio (compressed / decompressed bytes) for a stream of data " +
			"(as opposed to being calculated on a per-chunk basis for data in the stream)",
	}, []string{
		CacheNameLabel,
		CompressionType,
	})

	// Taken together, CompressedBlobSize and DecompressedBlobSize allow
	// determining the average overall compression ratio of items written to
	// the cache. This is an alternative to CompressionRatio (above) which
	// is skewed because it weights blobs of different sizes equally. These
	// values only track the blob size of blobs *written* to the cache.
	CompressedBlobSizeWrite = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "compressor",
		Name:      "compressed_blob_size_write",
		Help:      "The number of compressed bytes in all blobs",
	}, []string{
		CacheNameLabel,
		CompressionType,
	})
	DecompressedBlobSizeWrite = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "compressor",
		Name:      "decompressed_blob_size_write",
		Help:      "The number of decompressed bytes in all blobs",
	}, []string{
		CacheNameLabel,
		CompressionType,
	})

	// #### Examples
	//
	// ```promql
	// # Histogram buckets with the count of elements in each compression ratio bucket
	// # Visualize with the Bar Gauge type
	// # Legend: {{le}}
	// # Format: Heatmap
	// sum(increase(buildbuddy_pebble_compression_ratio_bucket[5m])) by(le)
	//
	// # Percentage of elements that increased in size when compressed (compression ratio > 1)
	// # Visualize with the Stat type
	// (sum(buildbuddy_pebble_compression_ratio_count) - sum(buildbuddy_pebble_compression_ratio_bucket{le="1.0"})) / sum(buildbuddy_pebble_compression_ratio_count)
	// ```

	ServerUploadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "upload_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes uploaded to the server in each upload. Use the **`_sum`** suffix to get the total uploaded bytes and the **`_count`** suffix to get the number of uploaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
	})

	ServerUncompressedUploadBytesCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "uncompressed_upload_bytes_count",
		Help:      "Number of uncompressed bytes uploaded to the server.",
	}, []string{
		CacheTypeLabel,
		ServerName,
		GroupID,
	})

	ServerDownloadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "download_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes downloaded from the server in each download. Use the **`_sum`** suffix to get the total downloaded bytes and the **`_count`** suffix to get the number of downloaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
		GroupID,
	})

	ServerUncompressedDownloadBytesCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "uncompressed_download_bytes_count",
		Help:      "Number of uncompressed bytes downloaded from the server.",
	}, []string{
		CacheTypeLabel,
		ServerName,
		GroupID,
	})

	DigestUploadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "digest_upload_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Digest size uploaded to the server in each upload. This does not always match the actual size uploaded to the server, if the client sends compressed bytes. Use the **`_sum`** suffix to get the total uploaded bytes and the **`_count`** suffix to get the number of uploaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
	})

	DigestDownloadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "server",
		Name:      "digest_download_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Digest size downloaded from the server in each download. This does not always match the actual size downloaded to the server, if the client requests compressed bytes. Use the **`_sum`** suffix to get the total downloaded bytes and the **`_count`** suffix to get the number of downloaded files.",
	}, []string{
		CacheTypeLabel,
		ServerName,
	})

	Logs = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "logger",
		Name:      "log_count",
		Help:      "The number of logs",
	}, []string{
		StatusHumanReadableLabel,
	})

	// ### Raft cache metrics

	RaftRanges = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "ranges",
		Help:      "Number of raft ranges on each nodehost.",
	}, []string{
		RaftNodeHostIDLabel,
	})

	RaftLeases = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "leases",
		Help:      "Number of raft leases on each nodehost.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftLeaders = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "leaders",
		Help:      "Number of raft leaders on each nodehost.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "bytes",
		Help:      "Size (in bytes) of each range.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftProposals = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "proposals",
		Help:      "The total number of statemachine proposals on each range.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftSplits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "splits",
		Help:      "The total number of splits per nodehost.",
	}, []string{
		RaftNodeHostIDLabel,
		StatusHumanReadableLabel,
	})

	RaftMoves = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "moves",
		Help:      "The total number of moves per nodehost.",
	}, []string{
		RaftNodeHostIDLabel,
		RaftMoveLabel,
		StatusHumanReadableLabel,
	})

	RaftRangeCacheLookups = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "rangecache_lookups",
		Help:      "The total number of rangecache lookups per nodehost.",
	}, []string{
		RaftRangeCacheEventTypeLabel,
	})

	RaftSplitDurationUs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "split_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent splitting a range in **microseconds**.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftReplicaUpdateDurationUs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "replica_update_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The time spent on replica.Update in **microseconds**.",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftEvictionErrorCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "eviction_errors",
		Help:      "The total number of eviction errors",
	})

	RaftListenerEventsDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "listener_events_dropped",
		Help:      "The total number of dropped listener events",
	}, []string{
		RaftListenerID,
		RaftListenerEventType,
	})

	RaftStoreEventsChanSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "store_events",
		Help:      "Number of events in the queue",
	})

	RaftStoreEventBroadcastDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "store_event_broadcast_dropped",
		Help:      "The total number of dropped events to broadcast",
	}, []string{
		RaftEventBroadcaster,
		RaftEventType,
	})

	RaftStoreEventListenerDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "store_event_listener_dropped",
		Help:      "The total number of dropped events in store",
	}, []string{
		RaftListenerID,
		RaftEventType,
	})

	RaftLeaseActionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "lease_action_count",
		Help:      "The total number of lease actions",
	}, []string{
		RaftRangeIDLabel,
		RaftLeaseActionLabel,
		StatusHumanReadableLabel,
	})

	RaftLeaseActionDurationMsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "lease_action_duration_msec",
		Buckets:   durationMsecBuckets(1*time.Millisecond, 15*time.Second, 2),
		Help:      "The duration of a lease action",
	}, []string{
		RaftLeaseActionLabel,
	})

	RaftZombieCleanupTasks = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "zombie_cleanup_tasks",
		Help:      "The total number of pending zombie cleanup tasks",
	})

	RaftZombieCleanup = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "zombie_cleanup",
		Help:      "The total number of zombie cleanups",
	}, []string{
		StatusHumanReadableLabel,
	})

	RaftRangeLockDurationMsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "range_lock_duration_msec",
		Buckets:   durationMsecBuckets(1*time.Millisecond, 15*time.Second, 2),
		Help:      "The duration of the range lock in a client session",
	}, []string{
		RaftRangeIDLabel,
	})

	RaftNodeHostMethodErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "nodehost_method_errors",
		Help:      "The total number of nodehost method",
	}, []string{
		RaftNodeHostMethodLabel,
		RaftDragonboatError,
	})

	RaftNodeHostMethodDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "nodehost_method_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The duration of a nodehost method",
	}, []string{
		RaftNodeHostMethodLabel,
		RaftRangeIDLabel,
	})

	RaftDriverActionCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "driver_actions",
		Help:      "The total number of driver action",
	}, []string{
		RaftDriverAction,
		RaftDriverRequeueType,
		RaftRangeIDLabel,
	})

	RaftIntermediateReplicaCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "intermediate_replicas_count",
		Help:      "The number of intermediate replicas",
	}, []string{
		RaftReplicaState,
		RaftRangeIDLabel,
	})

	RaftTxnRecordProcessCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "raft",
		Name:      "txn_record_process_count",
		Help:      "The total number of driver action",
	}, []string{
		RaftTxnRecordProcessStatus,
	})

	APIKeyLookupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "auth",
		Name:      "api_key_lookup_count",
		Help:      "Total number of API key lookups.",
	}, []string{
		APIKeyLookupStatus,
	})

	IPRulesCheckLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "auth",
		Name:      "ip_rules_check_latency_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 5*time.Second, 2),
		Help:      "Latency of IP authorization checks.",
	}, []string{
		StatusHumanReadableLabel,
	})

	EncryptionKeyRefreshCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "key_refresh_count",
		Help:      "Total number of encryption key refresh attempts.",
	})

	EncryptionKeyRefreshFailureCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "key_refresh_failure_count",
		Help:      "Total number of unsuccessful encryption key refresh attempts.",
	})

	EncryptionEncryptedBlockCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "encrypted_block_count",
		Help:      "Total number of blocks encrypted.",
	})

	EncryptionEncryptedBlobCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "encrypted_blob_count",
		Help:      "Total number of blobs encrypted.",
	})

	EncryptionDecryptedBlockCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "decrypted_block_count",
		Help:      "Total number of blocks decrypted.",
	})

	EncryptionDecryptedBlobCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "decrypted_blob_count",
		Help:      "Total number of blobs decrypted.",
	})

	EncryptionDecryptionErrorCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "decryption_error_count",
		Help:      "Total number of decryption errors.",
	})

	// This metric is in milliseconds because Grafana heatmaps don't display
	// microsecond durations nicely when they can contain large durations.
	EncryptionKeyLastEncryptedAgeMsec = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "encryption",
		Name:      "key_last_encryption_age_msec",
		Buckets:   exponentialBucketRange(float64(1*time.Hour.Milliseconds()), float64(7*24*time.Hour.Milliseconds()), 4),
		Help:      "Age of encrypted keys (i.e. how long it has been since the keys were re-encrypted).",
	})

	PebbleCacheAtimeUpdateCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_atime_update_count",
		Help:      "Count of processed atime updates.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	PebbleCacheAtimeUpdateGCSErrorCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_atime_update_gcs_error_count",
		Help:      "Count of atime update errors from GCS.",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	PebbleCacheAtimeDeltaWhenRead = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_atime_delta_when_read",
		Buckets: customDurationBuckets(
			time.Duration.Milliseconds,
			[]time.Duration{
				1 * time.Minute,
				5 * time.Minute,
				10 * time.Minute,
				30 * time.Minute,
				1 * time.Hour,
				3 * time.Hour,
				6 * time.Hour,
				12 * time.Hour,
				1 * day,
				2 * day,
				3 * day,
				4 * day,
				5 * day,
				6 * day,
				7 * day,
				14 * day,
				21 * day,
			}),
		Help: "Previous atime of items in the cache when they are read, in msec",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheEvictionSamplesChanSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_eviction_samples_chan_size",
		Help:      "Num of items in eviction samples chan",
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	PebbleCacheEvictionResampleLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_eviction_resample_latency_usec",
		Help:      "Latency of resampling during a single eviction iteration.",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 5*time.Second, 2),
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	PebbleCacheEvictionEvictLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_eviction_evict_latency_usec",
		Help:      "Latency of evicting a single key.",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 5*time.Second, 2),
	}, []string{
		PartitionID,
		CacheNameLabel,
	})

	PebbleCachePebbleCompactCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_compact_count",
		Help:      "Number of compactions performed by the underlying Pebble database.",
	}, []string{
		CompactionType,
		CacheNameLabel,
	})

	PebbleCachePebbleCompactEstimatedDebtBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_compact_estimated_debt_bytes",
		Help:      "Estimated number of bytes that need to be compacted for the LMS to reach a stable state.",
	}, []string{
		CacheNameLabel,
	})

	PebbleCachePebbleCompactInProgressBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_compact_in_progress_bytes",
		Help:      "Number of bytes present in sstables being written by in-progress compactions.",
	}, []string{
		CacheNameLabel,
	})

	PebbleCachePebbleCompactInProgress = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_compact_in_progress",
		Help:      "Number of compactions that are in-progress",
	}, []string{
		CacheNameLabel,
	})

	PebbleCachePebbleCompactMarkedFiles = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_compact_marked_files",
		Help:      "Count of files that are marked for compaction.",
	}, []string{
		CacheNameLabel,
	})

	PebbleCachePebbleLevelSublevels = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_sublevels",
		Help:      "Number of sublevels within the level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelNumFiles = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_num_files",
		Help:      "The total number of files in the level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_size_bytes",
		Help:      "The total size in bytes of the files in the level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_score",
		Help:      "The level's compaction score.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesInCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_in_count",
		Help:      "The number of incoming bytes from other levels read during compactions. This excludes bytes moved and bytes ingested. For L0 this is the bytes written to the WAL.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesIngestedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_ingested_count",
		Help:      "The number of bytes ingested.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesMovedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_moved_count",
		Help:      "The number of bytes moved into the level by a move compaction.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesReadCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_read_count",
		Help:      "The number of bytes read for compactions at the level. This includes bytes read from other levels (BytesIn), as well as bytes read for the level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesCompactedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_compacted_count",
		Help:      "The number of bytes written during compactions.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelBytesFlushedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_bytes_flushed_count",
		Help:      "The number of bytes written during flushes.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelTablesCompactedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_tables_compacted_count",
		Help:      "The number of sstables compacted to this level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelTablesFlushedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_tables_flushed_count",
		Help:      "The number of sstables flushed to this level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelTablesIngestedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_tables_ingested_count",
		Help:      "The number of sstables ingested into this level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleLevelTablesMovedCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_level_tables_moved_count",
		Help:      "The number of sstables ingested into to this level.",
	}, []string{
		PebbleLevel,
		CacheNameLabel,
	})

	PebbleCachePebbleOpCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_op_count",
		Help:      "The number of operations performed against the pebble database.",
	}, []string{
		PebbleID,
		PebbleOperation,
	})

	PebbleCachePebbleOpLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_op_latency_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 30*time.Second, 10),
		Help:      "The latency of operations performed against the pebble database, in microseconds.",
	}, []string{
		PebbleID,
		PebbleOperation,
	})

	// Total size of cache that pebble allocate mamually from system memory using malloc.
	PebbleCachePebbleBlockCacheSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_block_cache_size_bytes",
		Help:      "The total size in pebble's block cache.",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheWriteStallCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_write_stall_count",
		Help:      "The number of write stalls",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheWriteStallDurationUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_pebble_write_stall_duration_usec",
		Buckets:   coarseMicrosecondToHour,
		Help:      "The duration of write stall in pebble, in microseconds.",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheZombieTableCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_zombie_table_count",
		Help:      "The number of zombie tables",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheZombieTableSizeBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_zombie_table_size_bytes",
		Help:      "The size of zombie tables in bytes",
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheNumChunksPerFile = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_num_chunks_per_file",
		Help:      "Number of chunks per file stored in pebble cache",
		Buckets:   []float64{1.0, 10.0, 50.0, 100.0, 500.0, 1000.0, 1500.0, 2000.0, 2500.0},
	}, []string{
		CacheNameLabel,
	})

	PebbleCacheFindMissingDigestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_cache",
		Name:      "pebble_cache_find_missing_digest_count",
		Help:      "Count of digests within FindMissing requests.",
	}, []string{
		CacheNameLabel,
	})

	// ## Podman metrics

	PodmanSociStoreCrashes = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "podman",
		Name:      "soci_store_crash_count",
		Help:      "Total number of times the soci store binary crashed and was restarted.",
	})

	PodmanGetSociArtifactsLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "podman",
		Name:      "get_soci_artifacts_latency_usec",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 100*time.Minute, 10),
		Help:      "The latency of retrieving SOCI artifacts from the app and storing them locally per image, in microseconds. Note this is slightly different than the latency of the GetArtifacts RPC as the artifacts must be fetched from the cache and stored locally, which adds some additional time.",
	}, []string{
		ContainerImageTag,
	})

	PodmanGetSociArtifactsOutcomes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "podman",
		Name:      "get_soci_artifacts_outcome",
		Help:      "The outcome (cached or reason why not) of SociArtifactStore.GetArtifacts RPCs.",
	}, []string{
		GetSociArtifactsOutcomeTag,
	})

	PodmanColdImagePullLatencyMsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "podman",
		Name:      "image_pull_latency_msec",
		Buckets:   durationMsecBuckets(1*time.Millisecond, 100*time.Minute, 10),
		Help:      "The latency of 'cold' podman pull requests per image, in milliseconds. 'Cold' means the image hasn't been pulled by this executor previously.",
	}, []string{
		ContainerImageTag,
	})

	// ## Cache Proxy metrics
	ActionCacheProxiedReadRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "action_cache_read_requests",
		Help:      "The number of ActionCache.GetActionResult requests served by a ActionCacheServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ActionCacheProxiedWriteRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "action_cache_write_requests",
		Help:      "The number of ActionCache.UpdateActionResult requests served by a ActionCacheServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ActionCacheProxiedReadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "action_cache_read_bytes",
		Help:      "The number of ActionCache.GetActionResult bytes served by a ActionCacheServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ActionCacheProxiedWriteBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "action_cache_write_bytes",
		Help:      "The number of ActionCache.UpdateActionResult bytes served by a ActionCacheServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})

	ByteStreamProxiedReadRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "byte_stream_read_requests",
		Help:      "The number of ByteStream.Read requests served by a ByteStreamServerProxy broken down by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ByteStreamProxiedWriteRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "byte_stream_write_requests",
		Help:      "The number of ByteStream.Write requests served by a ByteStreamServerProxy broken down by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ByteStreamProxiedReadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "byte_stream_read_bytes",
		Help:      "The number of bytes read by ByteStream.Read RPCs served by a ByteStreamServerProxy broken down by gRPC status and cache hit/miss status. Note: this metric tracks bytes sent over the wire, which may be compressed.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})
	ByteStreamProxiedWriteBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "byte_stream_write_bytes",
		Help:      "The number of bytes written to a ByteStream.Write RPCs served by a ByteStreamServerProxy broken down by gRPC status and cache hit/miss status. Note: this metric tracks bytes sent over the wire, which may be compressed.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
		CacheProxyRequestType,
	})

	CapabilitiesProxiedRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "capabilities_requests",
		Help:      "The number of requests served by a CapabilitiesServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
	})
	CapabilitiesProxiedBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "capabilities_bytes",
		Help:      "The number of bytes served by a CapabilitiesServerProxy by gRPC status and cache hit/miss status.",
	}, []string{
		StatusLabel,
		CacheHitMissStatus,
	})

	// TODO(iain): consider adding gRPC status
	ContentAddressableStorageProxiedRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "content_addressable_storage_requests",
		Help:      "The number of requests served by a ContentAddressableStorageServerProxy by CAS operation and cache hit/miss status.",
	}, []string{
		CASOperation,
		CacheHitMissStatus,
	})

	ContentAddressableStorageProxiedDigests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "content_addressable_storage_digests",
		Help:      "The number of digests served by a ContentAddressableStorageServerProxy by CAS operation and cache hit/miss status.",
	}, []string{
		CASOperation,
		CacheHitMissStatus,
	})

	ContentAddressableStorageProxiedBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "content_addressable_storage_bytes",
		Help:      "The number of bytes served by a ContentAddressableStorageServerProxy by CAS operation and cache hit/miss status.",
	}, []string{
		CASOperation,
		CacheHitMissStatus,
	})

	RemoteAtimeUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "remote_atime_updates",
		Help:      "The number of remote atime updates enqueued, with the outcome of the enqueue operation.",
	}, []string{
		GroupID,
		EnqueueUpdateOutcome,
	})

	RemoteAtimeUpdatesSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "remote_atime_update_requests",
		Help:      "The number of FindMissingBlobRequests sent to the remote cache to update remote blob atimes.",
	}, []string{
		GroupID,
		StatusLabel,
	})

	RemoteHitTrackerUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "remote_hit_tracker_updates",
		Help:      "The number of remote hit-tracker updates enqueued, with the outcome of the enqueue operation.",
	}, []string{
		GroupID,
		EnqueueUpdateOutcome,
	})

	RemoteHitTrackerRequests = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "proxy",
		Name:      "remote_hit_tracker_requests",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 20), // 2^19 = 524,288
		Help:      "The number of HitTrackerService.Track RPCs sent to the remote hit-tracker service to record cache proxy cache hits. Histogram values are the number of hit-updates per request.",
	}, []string{
		GroupID,
		StatusLabel,
	})

	OCIRegistryCacheDownloadSizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "ociregistry",
		Name:      "cache_download_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(1, 10, 9),
		Help:      "Number of bytes downloaded from the cache by the OCI registry mirror",
	}, []string{
		OCIResourceTypeLabel,
	})

	OCIRegistryCacheEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: bbNamespace,
		Subsystem: "ociregistry",
		Name:      "cache_events",
		Help:      "Number of cache events handled.",
	}, []string{
		OCIResourceTypeLabel,
		CacheEventTypeLabel,
	})

	InputTreeSetupOpLatencyUsec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: bbNamespace,
		Subsystem: "remote_execution",
		Name:      "input_tree_setup_op_latency_usec",
		Help:      "Latency of individual operations used for setting up input trees.",
		Buckets:   durationUsecBuckets(1*time.Microsecond, 1*time.Second, 10),
	}, []string{
		OpLabel,
	})

	DiskFileWriterInProgressOps = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: bbNamespace,
		Subsystem: "disk",
		Name:      "file_writer_in_progress_ops",
		Help:      "Number of started, but not yet finished, FileWriter operations. This number includes operations that are blocked on the concurrency limiter.",
	})
)

// exponentialBucketRange returns prometheus.ExponentialBuckets specified in
// terms of a min and max value, rather than needing to explicitly calculate the
// number of buckets. You can use go/buckets (go.dev/play/p/-9T203IuxF1)
// to help you come up with sensible buckets for your metric.
func exponentialBucketRange(min, max, factor float64) []float64 {
	if min < 0 || min >= max {
		panic(fmt.Sprintf("exponentialBucketRange: expected 0 < min < max, got min=%f, max=%f", min, max))
	}
	if factor <= 1 {
		panic(fmt.Sprintf("exponentialBucketRange: expected factor > 1, got factor=%f", factor))
	}
	buckets := []float64{}
	current := min
	for current < max {
		buckets = append(buckets, current)
		current *= factor
	}
	return buckets
}

func durationUsecBuckets(min, max time.Duration, factor float64) []float64 {
	return exponentialBucketRange(float64(min.Microseconds()), float64(max.Microseconds()), factor)
}

func durationMsecBuckets(min, max time.Duration, factor float64) []float64 {
	return exponentialBucketRange(float64(min.Milliseconds()), float64(max.Milliseconds()), factor)
}

func customDurationBuckets(durationToUnit func(time.Duration) int64, durations []time.Duration) []float64 {
	buckets := []float64{}
	for _, d := range durations {
		buckets = append(buckets, float64(durationToUnit(d)))
	}
	return buckets
}
