package main

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kingpin"
	log "github.com/cihub/seelog"
	"github.com/goamz/goamz/aws"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	credsRegex *regexp.Regexp = regexp.MustCompile("^/(.+?)/meta-data/iam/security-credentials/(.*)$")

	instanceServiceClient *http.Transport = &http.Transport{}
)

var (
	configFile = kingpin.
		Flag("configfile", "Configuration file").
		Short('c').
		Required().
		String()
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

func configureLogging(config *LoggingConfig) {
	logger, err := log.LoggerFromConfigAsString(fmt.Sprintf(`
<seelog minlevel="%s">
    <outputs formatid="out">
        <console />
    </outputs>

    <formats>
        <format id="out" format="%%Date %%Time [%%LEVEL] %%Msg%%n" />
    </formats>
</seelog>
`, config.Level))

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

func NewGET(path string) *http.Request {
	r, err := http.NewRequest("GET", path, nil)

	if err != nil {
		panic(err)
	}

	return r
}

func handleCredentials(baseUrl, apiVersion, subpath string, c *CredentialsProvider, w http.ResponseWriter, r *http.Request) {
	resp, err := instanceServiceClient.RoundTrip(NewGET(baseUrl + "/" + apiVersion + "/meta-data/iam/security-credentials/"))

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
		creds, err := json.Marshal(&MetadataCredentials{
			Code:            "Success",
			LastUpdated:     credentials.GeneratedAt,
			Type:            "AWS-HMAC",
			AccessKeyId:     credentials.AccessKey,
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

func main() {
	kingpin.CommandLine.Help = "Docker container EC2 metadata service."
	kingpin.Parse()

	defer log.Flush()

	config, err := LoadConfigFile(*configFile)

	if err != nil {
		panic(err)
	}

	configureLogging(&config.Log)

	// Create auth object to query local metadata service for credentials
	auth, err := aws.GetAuth("", "", "", time.Time{})

	if err != nil {
		panic(err)
	}

	defaultRole, err := NewRoleArn(config.DefaultRole)

	if err != nil {
		panic(err)
	}

	platform, err := NewContainerService(config.Platform)

	if err != nil {
		panic(err)
	}

	credentials := NewCredentialsProvider(auth, platform, defaultRole)

	// Proxy non-credentials requests to primary metadata service
	http.HandleFunc("/", logHandler(func(w http.ResponseWriter, r *http.Request) {
		match := credsRegex.FindStringSubmatch(r.URL.Path)
		if match != nil {
			handleCredentials(config.Metadata.Url, match[1], match[2], credentials, w, r)
			return
		}

		proxyReq, err := http.NewRequest(r.Method, fmt.Sprintf("%s%s", config.Metadata.Url, r.URL.Path), r.Body)

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

	log.Critical(http.ListenAndServe(config.Bind.Addr(), nil))
}
