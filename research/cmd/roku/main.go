package main

import (
   "flag"
   "github.com/89z/mech/roku"
   "os"
   "path/filepath"
)

func main() {
   home, err := os.UserHomeDir()
   if err != nil {
      panic(err)
   }
   // b
   var mediaID string
   flag.StringVar(&mediaID, "b", "", "media ID")
   ///////////////////////////////////////
   var down downloader
   // a
   var address string
   flag.StringVar(&address, "a", "", "address")
   // c
   down.client = filepath.Join(home, "mech/client_id.bin")
   flag.StringVar(&down.client, "c", down.client, "client ID")
   // d
   var isDASH bool
   flag.BoolVar(&isDASH, "d", false, "DASH download")
   // f
   // therokuchannel.roku.com/watch/597a64a4a25c5bf6af4a8c7053049a6f
   var video int64
   flag.Int64Var(&video, "f", 1920832, "video bandwidth")
   // g
   // therokuchannel.roku.com/watch/597a64a4a25c5bf6af4a8c7053049a6f
   var audio int64
   flag.Int64Var(&audio, "g", 128000, "audio bandwidth")
   // i
   flag.BoolVar(&down.info, "i", false, "information")
   // k
   down.pem = filepath.Join(home, "mech/private_key.pem")
   flag.StringVar(&down.pem, "k", down.pem, "private key")
   // v
   var verbose bool
   flag.BoolVar(&verbose, "v", false, "verbose")
   flag.Parse()
   if verbose {
      roku.LogLevel = 1
   }
   if mediaID != "" || address != "" {
      if mediaID == "" {
         mediaID = roku.ContentID(address)
      }
      down.Content, err = roku.NewContent(mediaID)
      if err != nil {
         panic(err)
      }
      if isDASH {
         err := down.DASH(video, audio)
         if err != nil {
            panic(err)
         }
      } else {
         err := down.HLS(video)
         if err != nil {
            panic(err)
         }
      }
   } else {
      flag.Usage()
   }
}