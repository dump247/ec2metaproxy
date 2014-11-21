package main

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kingpin"
	log "github.com/cihub/seelog"
	"github.com/fsouza/go-dockerclient"
	"github.com/goamz/goamz/aws"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	baseUrl = "http://169.254.169.254" // no trailing slash '/'
)

var (
	credsRegex *regexp.Regexp = regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

	instanceServiceClient *http.Transport = &http.Transport{}
)

var (
	defaultRole = RoleArnOpt(kingpin.
			Flag("default-iam-role", "ARN of the role to use if the container does not specify a role.").
			Short('r'))

	serverAddr = kingpin.
			Flag("server", "Interface and port to bind the server to.").
			Default(":18000").
			Short('s').
			String()

	verboseOpt = kingpin.
			Flag("verbose", "Enable verbose output.").
			Bool()
)

type MetadataCredentials struct {
	Code            string
	LastUpdated     time.Time
	Type            string
	AccessKeyId     string
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

type RoleArnValue RoleArn

func (t *RoleArnValue) Set(value string) error {
	if len(value) > 0 {
		arn, err := NewRoleArn(value)
		*(*RoleArn)(t) = arn
		return err
	}

	return nil
}

func (t *RoleArnValue) String() string {
	return ""
}

func RoleArnOpt(s kingpin.Settings) (target *RoleArn) {
	target = new(RoleArn)
	s.SetValue((*RoleArnValue)(target))
	return
}

func copyHeaders(dst, src http.Header) {
	for k, _ := range dst {
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
	} else {
		return addr[:index]
	}
}

type LogResponseWriter struct {
	Wrapped http.ResponseWriter
	Status  int
}

func (t *LogResponseWriter) Header() http.Header {
	return t.Wrapped.Header()
}

func (t *LogResponseWriter) Write(d []byte) (int, error) {
	return t.Wrapped.Write(d)
}

func (t *LogResponseWriter) WriteHeader(s int) {
	t.Wrapped.WriteHeader(s)
	t.Status = s
}

func logHandler(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logWriter := &LogResponseWriter{w, 200}

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

func dockerClient() *docker.Client {
	client, err := docker.NewClient("unix:///var/run/docker.sock")

	if err != nil {
		panic(err)
	}

	return client
}

func NewGET(path string) *http.Request {
	r, err := http.NewRequest("GET", path, nil)

	if err != nil {
		panic(err)
	}

	return r
}

func handleCredentials(apiVersion, subpath string, c *ContainerService, w http.ResponseWriter, r *http.Request) {
	resp, err := instanceServiceClient.RoundTrip(NewGET(baseUrl + "/" + apiVersion + "/meta-data/iam/security-credentials/"))

	if err != nil {
		log.Error("Error requesting creds path for API version ", apiVersion, ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return
	}

	clientIP := remoteIP(r.RemoteAddr)
	role, err := c.RoleForIP(clientIP)

	if err != nil {
		log.Error(clientIP, " ", err)
		http.Error(w, "An unexpected error getting container role", http.StatusInternalServerError)
		return
	}

	roleName := role.Arn.RoleName()

	if len(subpath) == 0 {
		w.Write([]byte(roleName))
	} else if !strings.HasPrefix(subpath, roleName) || (len(subpath) > len(roleName) && subpath[len(roleName)-1] != '/') {
		// An idiosyncrasy of the standard EC2 metadata service:
		// Subpaths of the role name are ignored. So long as the correct role name is provided,
		// it can be followed by a slash and anything after the slash is ignored.
		w.WriteHeader(http.StatusNotFound)
	} else {
		creds, err := json.Marshal(&MetadataCredentials{
			Code:            "Success",
			LastUpdated:     role.LastUpdated,
			Type:            "AWS-HMAC",
			AccessKeyId:     role.Credentials.AccessKey,
			SecretAccessKey: role.Credentials.SecretKey,
			Token:           role.Credentials.Token,
			Expiration:      role.Credentials.Expiration,
		})

		if err != nil {
			log.Error("Error marshaling credentials: ", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Write(creds)
		}
	}
}

func main() {
	kingpin.CommandLine.Help = "Docker container EC2 metadata service."
	kingpin.Parse()

	defer log.Flush()
	configureLogging(*verboseOpt)

	auth, err := aws.GetAuth("", "", "", time.Time{})

	if err != nil {
		panic(err)
	}

	containerService := NewContainerService(dockerClient(), *defaultRole, auth)

	// Proxy non-credentials requests to primary metadata service
	http.HandleFunc("/", logHandler(func(w http.ResponseWriter, r *http.Request) {
		match := credsRegex.FindStringSubmatch(r.URL.Path)
		if match != nil {
			handleCredentials(match[1], match[2], containerService, w, r)
			return
		}

		proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", baseUrl, r.URL.Path), r.Body)

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

		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Warn("Error copying response content from EC2 metadata service: ", err)
		}
	}))

	log.Critical(http.ListenAndServe(*serverAddr, nil))
}
