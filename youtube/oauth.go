package youtube

import (
   "encoding/json"
   "fmt"
   "net/http"
   "net/http/httputil"
   "net/url"
   "os"
   "strings"
)

const (
   clientID =
      "861556708454-d6dlm3lh05idd8npek18k6be8ba3oc68" +
      ".apps.googleusercontent.com"
   clientSecret = "SboVhoG9s0rNafixCSGGKXAT"
)

type OAuth struct {
   Device_Code string
   User_Code string
   Verification_URL string
}

func NewOAuth() (*OAuth, error) {
   data := url.Values{
      "client_id": {clientID},
      "scope": {"https://www.googleapis.com/auth/youtube"},
   }
   res, err := http.PostForm("https://oauth2.googleapis.com/device/code", data)
   if err != nil {
      return nil, err
   }
   defer res.Body.Close()
   o := new(OAuth)
   if err := json.NewDecoder(res.Body).Decode(o); err != nil {
      return nil, err
   }
   return o, nil
}

func (o OAuth) Exchange() (*Exchange, error) {
   data := url.Values{
      "client_id": {clientID},
      "client_secret": {clientSecret},
      "device_code": {o.Device_Code},
      "grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
   }
   res, err := http.PostForm("https://oauth2.googleapis.com/token", data)
   if err != nil {
      return nil, err
   }
   defer res.Body.Close()
   x := new(Exchange)
   if err := json.NewDecoder(res.Body).Decode(x); err != nil {
      return nil, err
   }
   return x, nil
}

type Exchange struct {
   Access_Token string
   Refresh_Token string
}

func (x *Exchange) Refresh() error {
   data := url.Values{
      "client_id": {clientID},
      "client_secret": {clientSecret},
      "grant_type": {"refresh_token"},
      "refresh_token": {x.Refresh_Token},
   }
   req, err := http.NewRequest(
      "POST", "https://oauth2.googleapis.com/token",
      strings.NewReader(data.Encode()),
   )
   if err != nil {
      return err
   }
   req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
   dump, err := httputil.DumpRequest(req, true)
   if err != nil {
      return err
   }
   os.Stdout.Write(dump)
   res, err := new(http.Transport).RoundTrip(req)
   if err != nil {
      return err
   }
   defer res.Body.Close()
   if res.StatusCode != http.StatusOK {
      return fmt.Errorf("status %v", res.Status)
   }
   return json.NewDecoder(res.Body).Decode(x)
}
