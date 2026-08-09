package platform

// APIBase prefixes every versioned API operation. It lives in platform (not
// server) so domain modules can reference it without importing the server
// package, which imports them.
const APIBase = "/api/v1"
