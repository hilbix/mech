// RARBG
package rarbg

import (
   "fmt"
   "github.com/89z/mech/ocr.space"
   "net/http"
   "os"
   "path/filepath"
   "time"
)

const (
   AJAXPHP = "/threat_defence_ajax.php"
   CaptchaPHP = "/threat_captcha.php"
   DefencePHP = "/threat_defence.php"
   Origin = "http://rarbg.to"
   Sleep = 4 * time.Second
   invert = "\x1b[7m"
   reset = "\x1b[m"
)

// This returns solution to the Captcha at the given path. After this, you will
// want to call IamHuman.
func Solve(php string) (solve string, err error) {
   fmt.Println(invert, "GET", reset, Origin + php)
   res, err := http.Get(Origin + php)
   if err != nil { return "", err }
   defer res.Body.Close()
   if res.StatusCode != http.StatusOK {
      return "", fmt.Errorf("status %v", res.Status)
   }
   capt := filepath.Join(os.TempDir(), "captcha.png")
   file, err := os.Create(capt)
   if err != nil { return "", err }
   file.ReadFrom(res.Body)
   // need to close before opening again, not after return
   file.Close()
   img, err := ocr.NewImage(capt)
   if err != nil { return "", err }
   return img.ParsedResults[0].ParsedText, nil
}
