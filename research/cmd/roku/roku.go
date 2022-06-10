package main

import (
   "fmt"
   "github.com/89z/format"
   "github.com/89z/format/dash"
   "github.com/89z/format/hls"
   "github.com/89z/mech"
   "github.com/89z/mech/roku"
   "io"
   "net/http"
   "net/url"
   "os"
)

type downloader struct {
   *dash.Period
   *roku.Content
   *url.URL
   client string
   info bool
   key []byte
   pem string
}

func (d *downloader) setKey() error {
   site, err := roku.NewCrossSite()
   if err != nil {
      return err
   }
   play, err := site.Playback(d.Meta.ID)
   if err != nil {
      return err
   }
   privateKey, err := os.ReadFile(d.pem)
   if err != nil {
      return err
   }
   clientID, err := os.ReadFile(d.client)
   if err != nil {
      return err
   }
   kID, err := d.Protection().KID()
   if err != nil {
      return err
   }
   d.key, err = play.Key(privateKey, clientID, kID)
   if err != nil {
      return err
   }
   return nil
}

func (d downloader) DASH(video, audio int64) error {
   if d.info {
      fmt.Println(d.Content)
   }
   videoDASH := d.Content.DASH()
   fmt.Println("GET", videoDASH.URL)
   res, err := http.Get(videoDASH.URL)
   if err != nil {
      return err
   }
   defer res.Body.Close()
   d.URL = res.Request.URL
   d.Period, err = dash.NewPeriod(res.Body)
   if err != nil {
      return err
   }
   if err := d.download(audio, dash.Audio); err != nil {
      return err
   }
   return d.download(video, dash.Video)
}

func (d *downloader) download(band int64, fn dash.PeriodFunc) error {
   if band == 0 {
      return nil
   }
   reps := d.Represents(fn)
   rep := reps.Represent(band)
   if d.info {
      for _, each := range reps {
         if each.Bandwidth == rep.Bandwidth {
            fmt.Print("!")
         }
         fmt.Println(each)
      }
   } else {
      if d.key == nil {
         err := d.setKey()
         if err != nil {
            return err
         }
      }
      ext, err := mech.ExtensionByType(rep.MimeType)
      if err != nil {
         return err
      }
      file, err := os.Create(d.Content.Base()+ext)
      if err != nil {
         return err
      }
      defer file.Close()
      init, err := rep.Initialization(d.URL)
      if err != nil {
         return err
      }
      fmt.Println("GET", init)
      res, err := http.Get(init.String())
      if err != nil {
         return err
      }
      defer res.Body.Close()
      if _, err := file.ReadFrom(res.Body); err != nil {
         return err
      }
      media, err := rep.Media(d.URL)
      if err != nil {
         return err
      }
      pro := format.ProgressChunks(file, len(media))
      for _, addr := range media {
         res, err := http.Get(addr.String())
         if err != nil {
            return err
         }
         pro.AddChunk(res.ContentLength)
         if err := dash.Decrypt(pro, res.Body, d.key); err != nil {
            return err
         }
         if err := res.Body.Close(); err != nil {
            return err
         }
      }
   }
   return nil
}
func (d downloader) HLS(bandwidth int64) error {
   video, err := d.Content.HLS()
   if err != nil {
      return err
   }
   fmt.Println("GET", video.URL)
   res, err := http.Get(video.URL)
   if err != nil {
      return err
   }
   defer res.Body.Close()
   master, err := hls.NewScanner(res.Body).Master()
   if err != nil {
      return err
   }
   stream := master.Streams.GetBandwidth(bandwidth)
   if !d.info {
      addr, err := res.Request.URL.Parse(stream.RawURI)
      if err != nil {
         return err
      }
      return downloadHLS(addr, d.Base())
   }
   fmt.Println(d.Content)
   for _, each := range master.Streams {
      if each.Bandwidth == stream.Bandwidth {
         fmt.Print("!")
      }
      fmt.Println(each)
   }
   return nil
}

func downloadHLS(addr *url.URL, base string) error {
   fmt.Println("GET", addr)
   res, err := http.Get(addr.String())
   if err != nil {
      return err
   }
   defer res.Body.Close()
   seg, err := hls.NewScanner(res.Body).Segment()
   if err != nil {
      return err
   }
   file, err := os.Create(base + hls.TS)
   if err != nil {
      return err
   }
   defer file.Close()
   pro := format.ProgressChunks(file, len(seg.Clear))
   for _, clear := range seg.Clear {
      addr, err := res.Request.URL.Parse(clear)
      if err != nil {
         return err
      }
      res, err := http.Get(addr.String())
      if err != nil {
         return err
      }
      pro.AddChunk(res.ContentLength)
      if _, err := io.Copy(pro, res.Body); err != nil {
         return err
      }
      if err := res.Body.Close(); err != nil {
         return err
      }
   }
   return nil
}

