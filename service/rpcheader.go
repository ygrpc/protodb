package service

// YgrpcErr to indicate rpc caller need error in rpc response and the response has error
const YgrpcErr = "Ygrpc-Err"

// YgrpcErrHeader to indicate rpc caller need error in rpc response header with header-key =
// set to 1 to indicate rpc caller need error
// if no such header, the response error will not set on header
const YgrpcErrHeader = "Ygrpc-Err-Header"

// YgrpcErrMax to indicate rpc caller need error in rpc response header and specify max length for error header, in bytes
// if YgrpcErrHeader is not set or <= 0, there is no limit
const YgrpcErrMax = "Ygrpc-Err-Max"
