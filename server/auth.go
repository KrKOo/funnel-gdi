package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/ohsu-comp-bio/funnel/config"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Authentication struct {
	basic map[string]bool
	oidc  *OidcConfig
}

var (
	errMissingMetadata    = status.Errorf(codes.InvalidArgument, "Missing metadata in the context")
	errTokenRequired      = status.Errorf(codes.Unauthenticated, "Basic/Bearer authorization token missing")
	errInvalidBasicToken  = status.Errorf(codes.PermissionDenied, "Basic-authentication failed")
	errInvalidBearerToken = status.Errorf(codes.PermissionDenied, "Bearer authorization token not accepted")
)

func NewAuthentication(creds []config.BasicCredential, oidc config.OidcAuth) *Authentication {
	basicCreds := make(map[string]bool)
	for _, cred := range creds {
		credBytes := []byte(cred.User + ":" + cred.Password)
		fullValue := "Basic " + base64.StdEncoding.EncodeToString(credBytes)
		basicCreds[fullValue] = true
	}

	return &Authentication{
		basic: basicCreds,
		oidc:  initOidcConfig(oidc),
	}
}

// Return a new gRPC interceptor function that authorizes RPCs.
func (a *Authentication) Interceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	// Case when authentication is not required:
	if len(a.basic) == 0 && a.oidc == nil {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, errTokenRequired
	}

	authorized := false
	authErr := errTokenRequired

	if strings.HasPrefix(values[0], "Basic ") {
		authErr = errInvalidBasicToken
		headerValue := values[0]
		authorized = a.basic[headerValue]
		if !authorized {
			fmt.Printf("Received Auth: %s (valid: %v)\n", values[0], authorized)
		}
	} else if a.oidc != nil {
		if strings.HasPrefix(values[0], "Bearer ") {
			authErr = errInvalidBearerToken
			jwtString := strings.TrimPrefix(values[0], "Bearer ")
			jwt := a.oidc.ParseJwt(jwtString)
			authorized = jwt != nil
		}
	}

	if !authorized {
		return nil, authErr
	}

	return handler(ctx, req)
}

// HTTP request handler for the /login endpoint. Initiates user authentication
// flow based on the configuration (OIDC, Basic, none).
func (a *Authentication) LoginHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
	} else if a.oidc != nil {
		a.oidc.HandleAuthCode(w, req)
	} else if len(a.basic) > 0 {
		a.handleBasicAuth(w, req)
	} else {
		http.Redirect(w, req, "/", http.StatusSeeOther)
	}
}

// HTTP request handler for the /login/token endpoint. In case of OIDC enabled,
// prints the JWT from the sent cookie. In all other cases, an empty HTTP 200
// response.
func (a *Authentication) EchoTokenHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
	} else if a.oidc != nil {
		a.oidc.EchoTokenHandler(w, req)
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	}
}

func (a *Authentication) handleBasicAuth(w http.ResponseWriter, req *http.Request) {
	// Check if provided value in the header is valid:
	if a.basic[req.Header.Get("Authorization")] {
		http.Redirect(w, req, "/", http.StatusSeeOther)
	} else {
		w.Header().Set("WWW-Authenticate", "Basic realm=Funnel")
		msg := "User authentication is required (Basic authentication with " +
			"username and password)"
		http.Error(w, msg, http.StatusUnauthorized)
	}
}
