package status

import (
	"context"
	"flag"
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

var LogErrorStackTraces = flag.Bool("app.log_error_stack_traces", false, "If true, stack traces will be printed for errors that have them.")

const stackDepth = 10

type wrappedError struct {
	error
	*stack
}

func (w *wrappedError) GRPCStatus() *status.Status {
	if se, ok := w.error.(interface {
		GRPCStatus() *status.Status
	}); ok {
		return se.GRPCStatus()
	}
	return status.New(codes.Unknown, "")
}

type StackTrace = errors.StackTrace
type stack []uintptr

func (s *stack) StackTrace() StackTrace {
	f := make([]errors.Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = errors.Frame((*s)[i])
	}
	return f
}

func callers() *stack {
	var pcs [stackDepth]uintptr
	n := runtime.Callers(3, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

func makeStatusError(code codes.Code, msg string, details ...protoadapt.MessageV1) error {
	var statusError error

	// If no details are provided (most common case) don't call WithDetails
	// since it allocates a new status proto.
	if len(details) > 0 {
		s := status.New(code, msg)
		s, err := s.WithDetails(details...)
		if err != nil {
			return InternalErrorf("add error details to error: %s", err)
		}
		statusError = s.Err()
	} else {
		statusError = status.Error(code, msg)
	}

	if !*LogErrorStackTraces {
		return statusError
	}
	return &wrappedError{
		statusError,
		callers(),
	}
}

func OK() error {
	return status.Error(codes.OK, "")
}
func CanceledError(msg string) error {
	return makeStatusError(codes.Canceled, msg)
}
func IsCanceledError(err error) bool {
	return status.Code(err) == codes.Canceled
}
func CanceledErrorf(format string, a ...interface{}) error {
	return CanceledError(fmt.Sprintf(format, a...))
}
func UnknownError(msg string) error {
	return makeStatusError(codes.Unknown, msg)
}
func IsUnknownError(err error) bool {
	return status.Code(err) == codes.Unknown
}
func UnknownErrorf(format string, a ...interface{}) error {
	return UnknownError(fmt.Sprintf(format, a...))
}
func InvalidArgumentError(msg string) error {
	return makeStatusError(codes.InvalidArgument, msg)
}
func IsInvalidArgumentError(err error) bool {
	return status.Code(err) == codes.InvalidArgument
}
func InvalidArgumentErrorf(format string, a ...interface{}) error {
	return InvalidArgumentError(fmt.Sprintf(format, a...))
}
func DeadlineExceededError(msg string) error {
	return makeStatusError(codes.DeadlineExceeded, msg)
}
func IsDeadlineExceededError(err error) bool {
	return status.Code(err) == codes.DeadlineExceeded
}
func DeadlineExceededErrorf(format string, a ...interface{}) error {
	return DeadlineExceededError(fmt.Sprintf(format, a...))
}
func NotFoundError(msg string) error {
	return makeStatusError(codes.NotFound, msg)
}
func IsNotFoundError(err error) bool {
	return status.Code(err) == codes.NotFound
}
func NotFoundErrorf(format string, a ...interface{}) error {
	return NotFoundError(fmt.Sprintf(format, a...))
}
func AlreadyExistsError(msg string) error {
	return makeStatusError(codes.AlreadyExists, msg)
}
func IsAlreadyExistsError(err error) bool {
	return status.Code(err) == codes.AlreadyExists
}
func AlreadyExistsErrorf(format string, a ...interface{}) error {
	return AlreadyExistsError(fmt.Sprintf(format, a...))
}
func PermissionDeniedError(msg string) error {
	return makeStatusError(codes.PermissionDenied, msg)
}
func IsPermissionDeniedError(err error) bool {
	return status.Code(err) == codes.PermissionDenied
}
func PermissionDeniedErrorf(format string, a ...interface{}) error {
	return PermissionDeniedError(fmt.Sprintf(format, a...))
}
func ResourceExhaustedError(msg string) error {
	return makeStatusError(codes.ResourceExhausted, msg)
}
func IsResourceExhaustedError(err error) bool {
	return status.Code(err) == codes.ResourceExhausted
}
func ResourceExhaustedErrorf(format string, a ...interface{}) error {
	return ResourceExhaustedError(fmt.Sprintf(format, a...))
}
func FailedPreconditionError(msg string) error {
	return makeStatusError(codes.FailedPrecondition, msg)
}
func IsFailedPreconditionError(err error) bool {
	return status.Code(err) == codes.FailedPrecondition
}
func FailedPreconditionErrorf(format string, a ...interface{}) error {
	return FailedPreconditionError(fmt.Sprintf(format, a...))
}
func AbortedError(msg string) error {
	return makeStatusError(codes.Aborted, msg)
}
func IsAbortedError(err error) bool {
	return status.Code(err) == codes.Aborted
}
func AbortedErrorf(format string, a ...interface{}) error {
	return AbortedError(fmt.Sprintf(format, a...))
}
func OutOfRangeError(msg string) error {
	return makeStatusError(codes.OutOfRange, msg)
}
func IsOutOfRangeError(err error) bool {
	return status.Code(err) == codes.OutOfRange
}
func OutOfRangeErrorf(format string, a ...interface{}) error {
	return OutOfRangeError(fmt.Sprintf(format, a...))
}
func UnimplementedError(msg string) error {
	return makeStatusError(codes.Unimplemented, msg)
}
func IsUnimplementedError(err error) bool {
	return status.Code(err) == codes.Unimplemented
}
func UnimplementedErrorf(format string, a ...interface{}) error {
	return UnimplementedError(fmt.Sprintf(format, a...))
}
func InternalError(msg string) error {
	return makeStatusError(codes.Internal, msg)
}
func IsInternalError(err error) bool {
	return status.Code(err) == codes.Internal
}
func InternalErrorf(format string, a ...interface{}) error {
	return InternalError(fmt.Sprintf(format, a...))
}
func UnavailableError(msg string) error {
	return makeStatusError(codes.Unavailable, msg)
}
func IsUnavailableError(err error) bool {
	return status.Code(err) == codes.Unavailable
}
func UnavailableErrorf(format string, a ...interface{}) error {
	return UnavailableError(fmt.Sprintf(format, a...))
}
func DataLossError(msg string) error {
	return makeStatusError(codes.DataLoss, msg)
}
func IsDataLossError(err error) bool {
	return status.Code(err) == codes.DataLoss
}
func DataLossErrorf(format string, a ...interface{}) error {
	return DataLossError(fmt.Sprintf(format, a...))
}
func UnauthenticatedError(msg string) error {
	return makeStatusError(codes.Unauthenticated, msg)
}
func IsUnauthenticatedError(err error) bool {
	return status.Code(err) == codes.Unauthenticated
}
func UnauthenticatedErrorf(format string, a ...interface{}) error {
	return UnauthenticatedError(fmt.Sprintf(format, a...))
}

// WrapError prepends additional context to an error description, preserving the
// underlying status code and error details.
func WrapError(err error, msg string) error {
	s, _ := status.FromError(err)

	// Preserve any details from the original error.
	decodedDetails := s.Details()
	details := make([]protoadapt.MessageV1, len(decodedDetails))
	for i, detail := range decodedDetails {
		if err, ok := detail.(error); ok {
			return InternalErrorf("unmarshal status detail: %v", err)
		}
		if pb, ok := detail.(protoadapt.MessageV1); ok {
			details[i] = pb
		} else {
			return InternalErrorf("unmarshal status detail: unrecognized detail type %T", detail)
		}
	}

	return makeStatusError(status.Code(err), fmt.Sprintf("%s: %s", msg, Message(err)), details...)
}

// Wrapf is the "Printf" version of `Wrap`.
func WrapErrorf(err error, format string, a ...interface{}) error {
	return WrapError(err, fmt.Sprintf(format, a...))
}

// WithReason returns a new error with a reason code attached to the given
// error. The reason code should be a unique, constant string identifier in
// UPPER_SNAKE_CASE that identifies the proximate cause of an error. This can be
// useful in cases where additional granularity is needed than just the gRPC
// status code.
func WithReason(err error, reason string) error {
	info := &errdetails.ErrorInfo{
		Reason: reason,
		Domain: "buildbuddy.io",
	}
	st := status.New(status.Code(err), Message(err))
	st, detailsErr := st.WithDetails(info)
	if detailsErr != nil {
		// Ideally we'd alert.UnexpectedEvent here but it'd cause a circular
		// dep. This should never happen in practice, but conservatively just
		// return an INTERNAL error for now.
		return InternalErrorf("add error details to error %q: %s", err, detailsErr)
	}
	return st.Err()
}

// Message extracts the error message from a given error, which for gRPC errors
// is just the "desc" part of the error.
func Message(err error) string {
	if err == nil {
		return ""
	}
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return err.Error()
}

// FromContextError converts ctx.Err() to the equivalent gRPC status error.
func FromContextError(ctx context.Context) error {
	s := status.FromContextError(ctx.Err())
	return status.ErrorProto(s.Proto())
}

// MetricsLabel returns an appropriate value for StatusHumanReadableLabel given
// an error from a gRPC request (which may be `nil`). See
// `StatusHumanReadableLabel` in server/metrics/metrics.go
func MetricsLabel(err error) string {
	// Check for client context errors first (context canceled, deadline
	// exceeded).
	s := status.FromContextError(err)
	if s.Code() != codes.Unknown {
		return s.Code().String()
	}
	// Return response error code (or network-level/framework-internal etc.
	// errors).
	return status.Code(err).String()
}
