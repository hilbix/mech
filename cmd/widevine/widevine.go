package main

import (
   "bytes"
   "errors"
   "github.com/89z/format"
   "github.com/89z/mech/widevine"
   "io"
   "net/http"
   "os"
   "strings"
)

var logLevel format.LogLevel

type flags struct {
   address string
   client string
   header string
   keyID string
   privateKey string
}

func (f flags) contents() (widevine.Contents, error) {
   var (
      client widevine.Client
      err error
   )
   client.ID, err = os.ReadFile(f.client)
   if err != nil {
      return nil, err
   }
   client.PrivateKey, err = os.ReadFile(f.privateKey)
   if err != nil {
      return nil, err
   }
   client.RawKeyID = f.keyID
   module, err := client.Module()
   if err != nil {
      return nil, err
   }
   buf, err := module.Marshal()
   if err != nil {
      return nil, err
   }
   req, err := http.NewRequest(
      "POST", f.address, bytes.NewReader(buf),
   )
   if err != nil {
      return nil, err
   }
   if f.header != "" {
      key, val, ok := strings.Cut(f.header, ":")
      if ok {
         req.Header.Set(key, val)
      }
   }
   logLevel.Dump(req)
   res, err := new(http.Transport).RoundTrip(req)
   if err != nil {
      return nil, err
   }
   defer res.Body.Close()
   if res.StatusCode != http.StatusOK {
      return nil, errors.New(res.Status)
   }
   buf, err = io.ReadAll(res.Body)
   if err != nil {
      return nil, err
   }
   return module.Unmarshal(buf)
}
