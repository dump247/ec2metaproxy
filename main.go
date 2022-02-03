package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/cihub/seelog"
)

var (
	credsRegex = regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

	instanceServiceClient = &http.Transport{}
)

var (
	defaultIamRole = roleArnOpt(kingpin.
			Flag("default-iam-role", "ARN of the role to use if the container does not specify a role.").
			Short('r'))

	defaultIamPolicy = kingpin.
				Flag("default-iam-policy", "Default IAM policy to apply if the container does not provide a custom role/policy.").
				Default("").
				String()

	metadataURL = kingpin.
			Flag("metadata-url", "URL of the real EC2 metadata service.").
			Default("http://169.254.169.254").
			String()

	serverAddr = kingpin.
			Flag("server", "Interface and port to bind the server to.").
			Default(":18000").
			Short('s').
			String()

	verbose = kingpin.
		Flag("verbose", "Enable verbose output.").
		Bool()

	dockerCommand = kingpin.Command("docker", "Run proxy for docker container manager.")

	dockerEndpoint = dockerCommand.
			Flag("docker-endpoint", "Endpoint to communicate with the docker daemon.").
			Default("unix:///var/run/docker.sock").
			String()

	flynnCommand = kingpin.Command("flynn", "Run proxy for flynn container manager.")

	flynnEndpoint = flynnCommand.
			Flag("flynn-endpoint", "Endpoint to communicate with the flynn host.").
			Default("http://127.0.0.1:1113").
			String()
)

type metadataCredentials struct {
	Code            string
	LastUpdated     time.Time
	Type            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

func copyHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}

	for k, v := range src {
		vCopy := make([]string, len(v))
		copy(vCopy, v)
		dst[k] = vCopy
	}
}

func configureLogging(verbose bool) {
	minLevel := "info"

	if verbose {
		minLevel = "trace"
	}

	logger, err := log.LoggerFromConfigAsString(fmt.Sprintf(`
<seelog minlevel="%s">
    <outputs formatid="out">
        <console />
    </outputs>
    <formats>
        <format id="out" format="%%Date %%Time [%%LEVEL] %%Msg%%n" />
    </formats>
</seelog>
`, minLevel))

	if err != nil {
		panic(err)
	}

	log.ReplaceLogger(logger)
}

func remoteIP(addr string) string {
	index := strings.Index(addr, ":")

	if index < 0 {
		return addr
	}

	return addr[:index]
}

type logResponseWriter struct {
	Wrapped http.ResponseWriter
	Status  int
}

func (t *logResponseWriter) Header() http.Header {
	return t.Wrapped.Header()
}

func (t *logResponseWriter) Write(d []byte) (int, error) {
	return t.Wrapped.Write(d)
}

func (t *logResponseWriter) WriteHeader(s int) {
	t.Wrapped.WriteHeader(s)
	t.Status = s
}

func logHandler(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logWriter := &logResponseWriter{w, 200}

		defer func() {
			if e := recover(); e != nil {
				log.Critical("Panic in request handler: ", e)
				logWriter.WriteHeader(http.StatusInternalServerError)
			}

			elapsed := time.Since(start)
			log.Infof("%s \"%s %s %s\" %d %s", remoteIP(r.RemoteAddr), r.Method, r.URL.Path, r.Proto, logWriter.Status, elapsed)
		}()

		handler(logWriter, r)
	}
}

func fetchMetadataToken() (string, error) {
	var token = ""
	req, err := http.NewRequest(http.MethodPut, "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		log.Error("Error making request for fetching metadata token: ", err)
		return token, err
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "300")
	resp, err := client.Do(req)
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error("Error reading response body from fetching metadata token: ", err)
		}
		token = string(bodyBytes)
	}
	defer resp.Body.Close()
	return token, err
}

func newGET(path string) *http.Request {
	token, err := fetchMetadataToken()
	if err != nil {
		panic(err)
	}

	r, err := http.NewRequest("GET", path, nil)
	r.Header.Set("X-aws-ec2-metadata-token", token)

	if err != nil {
		panic(err)
	}

	return r
}

func handleCredentials(baseURL, apiVersion, subpath string, c *credentialsProvider, w http.ResponseWriter, r *http.Request) {
	resp, err := instanceServiceClient.RoundTrip(newGET(baseURL + "/" + apiVersion + "/meta-data/iam/security-credentials/"))

	if err != nil {
		log.Error("Error requesting creds path for API version ", apiVersion, ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	clientIP := remoteIP(r.RemoteAddr)
	credentials, err := c.CredentialsForIP(clientIP)

	if err != nil {
		log.Error(clientIP, " ", err)
		http.Error(w, "An unexpected error getting container role", http.StatusInternalServerError)
		return
	}

	roleName := credentials.RoleArn.RoleName()

	if len(subpath) == 0 {
		w.Write([]byte(roleName))
	} else if !strings.HasPrefix(subpath, roleName) || (len(subpath) > len(roleName) && subpath[len(roleName)-1] != '/') {
		// An idiosyncrasy of the standard EC2 metadata service:
		// Subpaths of the role name are ignored. So long as the correct role name is provided,
		// it can be followed by a slash and anything after the slash is ignored.
		w.WriteHeader(http.StatusNotFound)
	} else {
		creds, err := json.Marshal(&metadataCredentials{
			Code:            "Success",
			LastUpdated:     credentials.GeneratedAt,
			Type:            "AWS-HMAC",
			AccessKeyID:     credentials.AccessKey,
			SecretAccessKey: credentials.SecretKey,
			Token:           credentials.Token,
			Expiration:      credentials.Expiration,
		})

		if err != nil {
			log.Error("Error marshaling credentials: ", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write(creds)
		}
	}
}

func newContainerService(platform string) (containerService, error) {
	switch platform {
	case "docker":
		return newDockerContainerService(*dockerEndpoint)
	case "flynn":
		return newFlynnContainerService(*flynnEndpoint)
	default:
		return nil, fmt.Errorf("Unknown container platform: %s", platform)
	}
}

func main() {
	kingpin.CommandLine.Help = "Docker container EC2 metadata service."
	command := kingpin.Parse()

	defer log.Flush()
	configureLogging(*verbose)

	platform, err := newContainerService(command)

	if err != nil {
		panic(err)
	}

	awsSession := session.New()
	credentials := newCredentialsProvider(awsSession, platform, *defaultIamRole, *defaultIamPolicy)

	// Proxy non-credentials requests to primary metadata service
	http.HandleFunc("/", logHandler(func(w http.ResponseWriter, r *http.Request) {
		match := credsRegex.FindStringSubmatch(r.URL.Path)
		if match != nil {
			handleCredentials(*metadataURL, match[1], match[2], credentials, w, r)
			return
		}

		proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", *metadataURL, r.URL.Path), r.Body)

		if err != nil {
			log.Error("Error creating proxy http request: ", err)
			http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
			return
		}

		copyHeaders(proxyReq.Header, r.Header)
		resp, err := instanceServiceClient.RoundTrip(proxyReq)

		if err != nil {
			log.Error("Error forwarding request to EC2 metadata service: ", err)
			http.Error(w, "An unexpected error occurred communicating with Amazon", http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Warn("Error copying response content from EC2 metadata service: ", err)
		}
	}))

	log.Info("Listening on ", *serverAddr)
	log.Critical(http.ListenAndServe(*serverAddr, nil))
}
