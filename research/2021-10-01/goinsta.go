package goinsta

import (
   "encoding/json"
   "errors"
   "fmt"
   "io"
   "io/ioutil"
   "net/http"
   "net/http/cookiejar"
   "os"
   "strconv"
   "sync"
   "time"
   neturl "net/url"
)

// Instagram represent the main API handler
//
// Timeline:     Represents instagram's main timeline.
// Profiles:     Represents instagram's user profile.
// Account:      Represents instagram's personal account.
// Collections:  Represents instagram's saved post collections.
// Searchbar:    Represents instagram's search.
// Activity:     Represents instagram's user activity and notifications.
// Feed:         Represents instagram's feed for e.g. user pages and hashtags.
// Contacts:     Represents instagram's sync with contact book.
// Inbox:        Represents instagram's messages.
// Locations:    Represents instagram's locations.
// Challenges:   Represents instagram's url challenges
// TwoFactorInfo Represents Instagram's 2FA login
//
// See Scheme section in README.md for more information.
//
// We recommend to use Export and Import functions after first Login.
//
// Also you can use SetProxy and UnsetProxy to set and unset proxy.
// Golang also provides the option to set a proxy using HTTP_PROXY env var.
//
type Instagram struct {
	user string
	pass string

	// device id: android-1923fjnma8123
	dID string
	// family device id, v4 uuid: 8b13e7b3-28f7-4e05-9474-358c6602e3f8
	fID string
	// uuid: 8493-1233-4312312-5123
	uuid string
	// rankToken
	rankToken string
	// token -- I think this is depricated, as I don't see any csrf tokens being used anymore, but not 100% sure
	token string
	// phone id v4 uuid: fbf767a4-260a-490d-bcbb-ee7c9ed7c576
	pid string
	// ads id: 5b23a92b-3228-4cff-b6ab-3199f531f05b
	adid string
	// challenge URL
	challengeURL string
	// pigeonSessionId
	psID string
	// contains header options set by Instagram
	headerOptions sync.Map
	// expiry of X-Mid cookie
	xmidExpiry int64
	// Public Key
	pubKey string
	// Public Key ID
	pubKeyID int
	// Device Settings
	device Device
	// User-Agent
	userAgent string
	// Account stores all personal data of the user and his/her options.
	Account *Account
	// Challenge stores the challenge info if provided
	Challenge *Challenge
	// TwoFactorInfo enabled 2FA
	TwoFactorInfo *TwoFactorInfo

	c *http.Client

	// Set to true to debug reponses
	Debug bool

	// Non-error message handlers.
	// By default they will be printed out, alternatively you can e.g. pass them to a logger
	infoHandler  func(...interface{})
	warnHandler  func(...interface{})
	debugHandler func(...interface{})
}

func defaultHandler(args ...interface{}) {
	fmt.Println(args...)
}

func (insta *Instagram) SetInfoHandler(f func(...interface{})) {
	insta.infoHandler = f
}

func (insta *Instagram) SetWarnHandler(f func(...interface{})) {
	insta.warnHandler = f
}

func (insta *Instagram) SetDebugHandler(f func(...interface{})) {
	insta.debugHandler = f
}

// SetHTTPClient sets http client.  This further allows users to use this functionality
// for HTTP testing using a mocking HTTP client Transport, which avoids direct calls to
// the Instagram, instead of returning mocked responses.
func (insta *Instagram) SetHTTPClient(client *http.Client) {
	insta.c = client
}

// SetHTTPTransport sets http transport. This further allows users to tweak the underlying
// low level transport for adding additional fucntionalities.
func (insta *Instagram) SetHTTPTransport(transport http.RoundTripper) {
	insta.c.Transport = transport
}

// SetDeviceID sets device id | android-1923fjnma8123
func (insta *Instagram) SetDeviceID(id string) {
	insta.dID = id
}

// SetUUID sets v4 uuid | 71cd1aec-e146-4380-8d60-d216127c7b4e
func (insta *Instagram) SetUUID(uuid string) {
	insta.uuid = uuid
}

// SetPhoneID sets phone id, v4 uuid | fbf767a4-260a-490d-bcbb-ee7c9ed7c576
func (insta *Instagram) SetPhoneID(id string) {
	insta.pid = id
}

// SetPhoneID sets phone family id, v4 uuid | 8b13e7b3-28f7-4e05-9474-358c6602e3f8
func (insta *Instagram) SetFamilyID(id string) {
	insta.fID = id
}

// SetAdID sets the ad id, v4 uuid |  5b23a92b-3228-4cff-b6ab-3199f531f05b
func (insta *Instagram) SetAdID(id string) {
	insta.adid = id
}

// SetDevice allows you to set a custom device. This will also change the
//   user agent based on the new device.
func (insta *Instagram) SetDevice(device Device) {
	insta.device = device
	insta.userAgent = createUserAgent(device)
}

// SetCookieJar sets the Cookie Jar. This further allows to use a custom implementation
// of a cookie jar which may be backed by a different data store such as redis.
func (insta *Instagram) SetCookieJar(jar http.CookieJar) error {
	url, err := neturl.Parse(instaAPIUrl)
	if err != nil {
		return err
	}
	// First grab the cookies from the existing jar and we'll put it in the new jar.
	cookies := insta.c.Jar.Cookies(url)
	insta.c.Jar = jar
	insta.c.Jar.SetCookies(url, cookies)
	return nil
}

// New creates Instagram structure
func New(username, password string) *Instagram {
	// this call never returns error
	jar, _ := cookiejar.New(nil)
	insta := &Instagram{
		user: username,
		pass: password,
		dID: generateDeviceID(
			generateMD5Hash(username + password),
		),
		uuid:          generateUUID(),
		pid:           generateUUID(),
		fID:           generateUUID(),
		psID:          "UFS-" + generateUUID() + "-0",
		headerOptions: sync.Map{},
		xmidExpiry:    -1,
		device:        GalaxyS10,
		userAgent:     createUserAgent(GalaxyS10),
		c: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
			Jar: jar,
		},
		infoHandler:  defaultHandler,
		warnHandler:  defaultHandler,
		debugHandler: defaultHandler,
	}

	for k, v := range defaultHeaderOptions {
		insta.headerOptions.Store(k, v)
	}

	return insta
}

// Export exports selected *Instagram object options to an io.Writer
func (insta *Instagram) ExportIO(writer io.Writer) error {
	url, err := neturl.Parse(instaAPIUrl)
	if err != nil {
		return err
	}

	config := ConfigFile{
		ID:            insta.Account.ID,
		User:          insta.user,
		DeviceID:      insta.dID,
		FamilyID:      insta.fID,
		UUID:          insta.uuid,
		RankToken:     insta.rankToken,
		Token:         insta.token,
		PhoneID:       insta.pid,
		XmidExpiry:    insta.xmidExpiry,
		HeaderOptions: map[string]string{},
		Cookies:       insta.c.Jar.Cookies(url),
		Account:       insta.Account,
		Device:        insta.device,
	}

	setHeaders := func(key, value interface{}) bool {
		config.HeaderOptions[key.(string)] = value.(string)
		return true
	}

	insta.headerOptions.Range(setHeaders)
	bytes, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = writer.Write(bytes)
	return err
}

// ImportReader imports instagram configuration from io.Reader
//
// This function does not set proxy automatically. Use SetProxy after this call.
func ImportReader(r io.Reader, args ...interface{}) (*Instagram, error) {
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	config := ConfigFile{}
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}
	return ImportConfig(config, args...)
}

// ImportConfig imports instagram configuration from a configuration object.
//
// This function does not set proxy automatically. Use SetProxy after this call.
func ImportConfig(config ConfigFile, args ...interface{}) (*Instagram, error) {
	url, err := neturl.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	insta := &Instagram{
		user:          config.User,
		dID:           config.DeviceID,
		fID:           config.FamilyID,
		uuid:          config.UUID,
		rankToken:     config.RankToken,
		token:         config.Token,
		pid:           config.PhoneID,
		xmidExpiry:    config.XmidExpiry,
		headerOptions: sync.Map{},
		device:        config.Device,
		c: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
		Account: config.Account,

		infoHandler:  defaultHandler,
		warnHandler:  defaultHandler,
		debugHandler: defaultHandler,
	}
	insta.userAgent = createUserAgent(insta.device)
	insta.c.Jar, err = cookiejar.New(nil)
	if err != nil {
		return insta, err
	}
	insta.c.Jar.SetCookies(url, config.Cookies)

	for k, v := range config.HeaderOptions {
		insta.headerOptions.Store(k, v)
	}

	dontSync := false
	if len(args) != 0 {
		switch v := args[0].(type) {
		case bool:
			dontSync = v
		}
	}

	if dontSync {
		insta.Account.insta = insta
	} else {
		insta.Account = &Account{
			insta: insta,
			ID:    config.ID,
		}
		err = insta.Account.Sync()
		if err != nil {
			return nil, err
		}
	}

	return insta, nil
}

// Import imports instagram configuration
//
// This function does not set proxy automatically. Use SetProxy after this call.
func Import(path string, args ...interface{}) (*Instagram, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ImportReader(f, args...)
}

// Login performs instagram login sequence in close resemblance to the android apk.
//
// Password will be deleted after login
func (insta *Instagram) Login() (err error) {
	// pre-login sequence
	err = insta.zrToken()
	if err != nil {
		return
	}
	err = insta.sync()
	if err != nil {
		return
	}

	err = insta.getPrefill()
	if err != nil {
		insta.warnHandler("Non fatal error while fetching prefill:", err)
	}

	err = insta.contactPrefill()
	if err != nil {
		insta.warnHandler("Non fatal error while fetching contact prefill:", err)
	}

	err = insta.sync()
	if err != nil {
		return
	}
	if insta.pubKey == "" || insta.pubKeyID == 0 {
		return errors.New("Sync returned empty public key and/or public key id")
	}
	return insta.login()
}

func (insta *Instagram) login() error {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	if insta.pubKey == "" || insta.pubKeyID == 0 {
		return errors.New(
			"No public key or public key ID set. Please call Instagram.Sync() and verify that it works correctly",
		)
	}
	encrypted, err := EncryptPassword(insta.pass, insta.pubKey, insta.pubKeyID, timestamp)
	if err != nil {
		return err
	}

	result, err := json.Marshal(
		map[string]interface{}{
			"jazoest":             jazoest(insta.dID),
			"country_code":        "[{\"country_code\":\"44\",\"source\":[\"default\"]}]",
			"phone_id":            insta.fID,
			"enc_password":        encrypted,
			"username":            insta.user,
			"adid":                insta.adid,
			"guid":                insta.uuid,
			"device_id":           insta.dID,
			"google_tokens":       "[]",
			"login_attempt_count": 0,
		},
	)
	if err != nil {
		return err
	}
	body, _, err := insta.sendRequest(
		&reqOptions{
			Endpoint: urlLogin,
			Query:    map[string]string{"signed_body": "SIGNATURE." + string(result)},
			IsPost:   true,
		},
	)

	insta.pass = ""
	if err != nil {
		return err
	}
	return insta.verifyLogin(body)
}

func (insta *Instagram) verifyLogin(body []byte) error {
	res := accountResp{}
	err := json.Unmarshal(body, &res)
	if err != nil {
		return fmt.Errorf("failed to parse json from login response with err: %s", err.Error())
	}

	if res.Status != "ok" {
		err := errors.New(
			fmt.Sprintf(
				"Failed to login: %s, %s",
				res.ErrorType, res.Message,
			),
		)
		insta.warnHandler(err)

		switch res.ErrorType {
		case "bad_password":
			return ErrBadPassword
		case "two_factor_required":
			insta.TwoFactorInfo = res.TwoFactorInfo
			insta.TwoFactorInfo.insta = insta
		case "checkpoint_challenge_required":
			insta.Challenge = res.Challenge
			insta.Challenge.insta = insta
		}
		return err
	}

	insta.Account = &res.Account
	insta.Account.insta = insta
	insta.rankToken = strconv.FormatInt(insta.Account.ID, 10) + "_" + insta.uuid

	return nil
}

func (insta *Instagram) getPrefill() error {
	data, err := json.Marshal(
		map[string]string{
			"android_device_id": insta.dID,
			"phone_id":          insta.fID,
			"usages":            "[\"account_recovery_omnibox\"]",
			"device_id":         insta.uuid,
		},
	)
	if err != nil {
		return err
	}

	// ignore the error returned by the request, because 429 if often returned.
	// request is non-critical.
	insta.sendRequest(
		&reqOptions{
			Endpoint: urlGetPrefill,
			IsPost:   true,
			Query:    generateSignature(data),
		},
	)
	return nil
}

func (insta *Instagram) contactPrefill() error {
	data, err := json.Marshal(
		map[string]string{
			"phone_id": insta.fID,
			"usage":    "prefill",
		},
	)
	if err != nil {
		return err
	}

	// ignore the error returned by the request, because 429 if often returned
	//   and body is not needed. Request is non-critical.
	insta.sendRequest(
		&reqOptions{
			Endpoint: urlContactPrefill,
			IsPost:   true,
			Query:    generateSignature(data),
		},
	)
	return nil
}

func (insta *Instagram) zrToken() error {
	body, _, err := insta.sendRequest(
		&reqOptions{
			Endpoint: urlZrToken,
			IsPost:   false,
			Query: map[string]string{
				"device_id":        insta.dID,
				"token_hash":       "",
				"custom_device_id": insta.uuid,
				"fetch_reason":     "token_expired",
			},
			IgnoreHeaders: []string{
				"X-Pigeon-Session-Id",
				"X-Pigeon-Rawclienttime",
				"X-Ig-App-Locale",
				"X-Ig-Device-Locale",
				"X-Ig-Mapped-Locale",
				"X-Ig-App-Startup-Country",
			},
		},
	)
	if err != nil {
		return nil
	}

	var res map[string]interface{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return err
	}

	// Get the expiry time of the token
	token := res["token"].(map[string]interface{})
	ttl := token["ttl"].(float64)
	t := token["request_time"].(float64)
	insta.xmidExpiry = int64(t + ttl)

	return err
}

func (insta *Instagram) sync(args ...map[string]string) error {
	var query map[string]string
	if insta.Account == nil {
		query = map[string]string{
			"id":                      insta.uuid,
			"server_config_retrieval": "1",
		}
	} else {
		// if logged in
		query = map[string]string{
			"id":                      toString(insta.Account.ID),
			"_id":                     toString(insta.Account.ID),
			"_uuid":                   insta.uuid,
			"server_config_retrieval": "1",
		}
	}
	data, err := json.Marshal(query)
	if err != nil {
		return err
	}

	_, h, err := insta.sendRequest(
		&reqOptions{
			Endpoint: urlSync,
			Query:    generateSignature(data),
			IsPost:   true,
			IgnoreHeaders: []string{
				"Authorization",
			},
		},
	)
	if err != nil {
		return err
	}

	hkey := h["Ig-Set-Password-Encryption-Pub-Key"]
	hkeyID := h["Ig-Set-Password-Encryption-Key-Id"]
	var key string
	var keyID string
	if len(hkey) > 0 && len(hkeyID) > 0 && hkey[0] != "" && hkeyID[0] != "" {
		key = hkey[0]
		keyID = hkeyID[0]
	}

	id, err := strconv.Atoi(keyID)
	if err != nil {
		insta.warnHandler(fmt.Errorf("Failed to parse public key id: %s", err))
	}
	insta.pubKey = key
	insta.pubKeyID = id

	return nil
}