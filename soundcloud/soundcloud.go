package soundcloud

import (
   "bytes"
   "encoding/json"
   "fmt"
   "io"
   "net/http"
   "net/url"
   "regexp"
   "strconv"
   "strings"
)

const (
   resolveURL = "https://api-v2.soundcloud.com/resolve"
   trackURL = "https://api-v2.soundcloud.com/tracks"
)

var urlRegex = regexp.MustCompile(`(?m)^https?:\/\/(soundcloud\.com)\/(.*)$`)

// IsURL returns true if the provided url is a valid SoundCloud URL
func IsURL(url string, testMobile, testFirebase bool) bool {
   success := false
   if !success {
      success = len(urlRegex.FindAllString(url, -1)) > 0
   }
   return success
}

func sliceContains(slice []int64, x int64) bool {
   for _, i := range slice {
      if i == x {
         return true
      }
   }
   return false
}

type Client struct {
   clientID string
   httpClient *http.Client
}

// New returns a pointer to a new SoundCloud API struct. First fetch a
// SoundCloud client ID. This algorithm is adapted from
// https://www.npmjs.com/package/soundcloud-key-fetch. The basic notion of how
// this function works is that SoundCloud provides a client ID so its web app
// can make API requests. This client ID (along with other intialization data
// for the web app) is provided in a JavaScript file imported through a
// <script> tag in the HTML. This function scrapes the HTML and tries to find
// the URL to that JS file, and then scrapes the JS file to find the client ID.
func New() (*Client, error) {
   resp, err := http.Get("https://soundcloud.com")
   if err != nil {
      return nil, err
   }
   body, err := io.ReadAll(resp.Body)
   if err != nil {
      return nil, err
   }
   bodyString := string(body)
   // The link to the JS file with the client ID looks like this:
   // <script crossorigin
   // src="https://a-v2.sndcdn.com/assets/sdfhkjhsdkf.js"></script
   split := strings.Split(bodyString, `<script crossorigin src="`)
   urls := []string{}
   // Extract all the URLS that match our pattern
   for _, raw := range split {
      u := strings.Replace(raw, `"></script>`, "", 1)
      u = strings.Split(u, "\n")[0]
      if string([]rune(u)[0:31]) == "https://a-v2.sndcdn.com/assets/" {
         urls = append(urls, u)
      }
   }
   // It seems like our desired URL is always imported last,
   // so we use urls[len(urls) - 1]
   resp, err = http.Get(urls[len(urls)-1])
   if err != nil {
      return nil, err
   }
   body, err = io.ReadAll(resp.Body)
   if err != nil {
      return nil, err
   }
   bodyString = string(body)
   // Extract the client ID
   if strings.Contains(bodyString, `,client_id:"`) {
      clientID := strings.Split(bodyString, `,client_id:"`)[1]
      clientID = strings.Split(clientID, `"`)[0]
      return &Client{clientID, http.DefaultClient}, nil
   }
   return nil, fmt.Errorf("%v fail", bodyString)
}

// GetDownloadURL retuns the URL to download a track. This is useful if you
// want to implement your own downloading algorithm. If the track has a
// publicly available download link, that link will be preferred and the
// streamType parameter will be ignored. streamType can be either "hls" or
// "progressive", defaults to "progressive"
func (sc Client) GetDownloadURL(url string, streamType string) (string, error) {
   streamType = strings.ToLower(streamType)
   if streamType == "" {
      streamType = "progressive"
   }
   if !IsURL(url, false, false) {
      return "", fmt.Errorf("%v is not a track URL", url)
   }
   info, err := sc.getTrackInfo(GetTrackInfoOptions{
      URL: url,
   })
   if err != nil {
      return "", err
   }
   if len(info) == 0 {
      return "", fmt.Errorf("%v fail", url)
   }
   if info[0].Downloadable && info[0].HasDownloadsLeft {
      downloadURL, err := sc.getDownloadURL(info[0].ID)
      if err != nil {
         return "", err
      }
      return downloadURL, nil
   }
   for _, transcoding := range info[0].Media.Transcodings {
      if strings.ToLower(transcoding.Format.Protocol) == streamType {
         mediaURL, err := sc.getMediaURL(transcoding.URL)
         if err != nil {
            return "", err
         }
         return mediaURL, nil
      }
   }
   mediaURL, err := sc.getMediaURL(info[0].Media.Transcodings[0].URL)
   if err != nil {
      return "", err
   }
   return mediaURL, nil
}

func (c *Client) buildURL(base string, clientID bool, query ...string) (string, error) {
   if len(query)%2 != 0 {
      return "", fmt.Errorf("invalid query %v", query)
   }
   u, err := url.Parse(string(base))
   if err != nil {
      return "", err
   }
   q := u.Query()
   for i := 0; i < len(query); i += 2 {
      q.Add(query[i], query[i+1])
   }
   if clientID {
      q.Add("client_id", c.clientID)
   }
   u.RawQuery = q.Encode()
   return u.String(), nil
}

// getDownloadURL gets the download URL of a publicly downloadable track
func (c *Client) getDownloadURL(id int64) (string, error) {
   u, err := c.buildURL(fmt.Sprintf("https://api-v2.soundcloud.com/tracks/%d/download", id), true)
   if err != nil {
      return "", err
   }
   data, err := c.makeRequest("GET", u, nil)
   if err != nil {
      return "", err
   }
   // DownloadURLResponse is the JSON respose of retrieving media information
   // of a publicly downloadable track
   var res struct {
      RedirectURI string
   }
   if err := json.Unmarshal(data, &res); err != nil {
      return "", err
   }
   return res.RedirectURI, nil
}

func (c *Client) getMediaURL(url string) (string, error) {
   // The media URL is the actual link to the audio file for the track
   u, err := c.buildURL(url, true)
   if err != nil {
   return "", err
   }
   // MediaURLResponse is the JSON response of retrieving media information of a
   // track
   type MediaURLResponse struct {
      URL string
   }
   media := &MediaURLResponse{}
   data, err := c.makeRequest("GET", u, nil)
   if err != nil {
      return "", err
   }
   err = json.Unmarshal(data, media)
   if err != nil {
      return "", err
   }
   return media.URL, nil
}

func (c *Client) getTrackInfo(opts GetTrackInfoOptions) ([]Track, error) {
   var (
      data []byte
      err error
      trackInfo []Track
      u string
   )
   if opts.ID != nil && len(opts.ID) > 0 {
      ids := []string{}
      for _, id := range opts.ID {
         ids = append(ids, strconv.FormatInt(id, 10))
      }
      if opts.PlaylistID == 0 && opts.PlaylistSecretToken == "" {
         u, err = c.buildURL(trackURL, true, "ids", strings.Join(ids, ","))
      } else {
         u, err = c.buildURL(
            trackURL, true, "ids", strings.Join(ids, ","), "playlistId",
            fmt.Sprintf("%d", opts.PlaylistID), "playlistSecretToken",
            opts.PlaylistSecretToken,
         )
      }
      if err != nil {
         return nil, err
      }
      data, err = c.makeRequest("GET", u, nil)
      if err != nil {
         return nil, err
      }
      err = json.Unmarshal(data, &trackInfo)
      if err != nil {
         return nil, err
      }
   } else if opts.URL != "" {
      data, err = c.resolve(opts.URL)
      if err != nil {
         return nil, err
      }
      trackSingle := Track{}
      err = json.Unmarshal(data, &trackSingle)
      if err != nil {
         return nil, err
      }
      trackInfo = []Track{trackSingle}
   } else {
      return nil, fmt.Errorf("%v invalid", opts)
   }
   if opts.ID != nil && len(opts.ID) > 0 {
      trimmedIDs := []int64{}
      trackInfoIDs := []int64{}
      for _, track := range trackInfo {
         trackInfoIDs = append(trackInfoIDs, track.ID)
      }
      for _, id := range opts.ID {
         if sliceContains(trackInfoIDs, id) {
            trimmedIDs = append(trimmedIDs, id)
         }
      }
      c.sortTrackInfo(trimmedIDs, trackInfo)
   }
   return trackInfo, nil
}

func (c *Client) makeRequest(method, url string, body interface{}) ([]byte, error) {
   var (
      jsonBytes []byte
      err error
   )
   if body != nil {
      jsonBytes, err = json.Marshal(body)
      if err != nil {
         return nil, err
      }
   }
   req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonBytes))
   if err != nil {
      return nil, err
   }
   res, err := c.httpClient.Do(req)
   if err != nil {
      return nil, err
   }
   if res.StatusCode < 200 || res.StatusCode > 299 {
      return nil, fmt.Errorf("status %v", res.Status)
   }
   return io.ReadAll(res.Body)
}

// resolve is a handy API endpoint that returns info from the given resource
// URL
func (c *Client) resolve(url string) ([]byte, error) {
   u, err := c.buildURL(resolveURL, true, "url", strings.TrimRight(url, "/"))
   if err != nil {
      return nil, err
   }
   return c.makeRequest("GET", u, nil)
}

// Bubble Sort for now. Maybe switch to a more efficient sorting algorithm
// later?? Because the API request in getTrackInfo is limited to 50 tracks at
// once time complexity will always be <= O(50^2)
func (c *Client) sortTrackInfo(ids []int64, tracks []Track) {
   for j, id := range ids {
      if tracks[j].ID != id {
         for k := 0; k < len(tracks); k++ {
            if tracks[k].ID == id {
               temp := tracks[j]
               tracks[j] = tracks[k]
               tracks[k] = temp
            }
         }
      }
   }
}

// GetTrackInfoOptions can contain the URL of the track or the ID of the track.
// PlaylistID and PlaylistSecretToken are necessary to retrieve private tracks
// in private playlists.
type GetTrackInfoOptions struct {
   ID                  []int64
   PlaylistID          int64
   PlaylistSecretToken string
   URL                 string
}

// Track represents the JSON response of a track's info
type Track struct {
   Downloadable      bool
   HasDownloadsLeft  bool   `json:"has_downloads_left"`
   CreatedAt         string `json:"created_at"`
   Description       string
   DurationMS        int64  `json:"duration"`
   FullDurationMS    int64  `json:"full_duration"`
   ID                int64
   Kind string
   // Media contains an array of transcoding for a track
   Media struct {
      // Transcoding contains information about the transcoding of a track
      Transcodings []struct {
         // TranscodingFormat contains the protocol by which the track is
         // delivered ("progressive" or "HLS"), and the mime type of the track
         Format struct {
            MimeType string `json:"mime_type"`
            Protocol string
         }
         Preset  string
         Snipped bool
         URL     string
      }
   }
   Permalink string
   PermalinkURL string `json:"permalink_url"`
   PlaybackCount int64  `json:"playback_count"`
   SecretToken string `json:"secret_token"`
   Streamable bool
   Title string
   URI string
   WaveformURL string `json:"waveform_url"`
}
