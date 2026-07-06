package oembed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const videoBody = `{"type":"video","title":"A Video","author_name":"Chan",
	"thumbnail_url":"https://t.example/1.jpg",
	"html":"<iframe src=\"https://player.example/embed/1\"></iframe>"}`

func TestResolveVideo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(videoBody))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.Endpoints = map[string]string{"tube.example": srv.URL + "/oembed?url=%s"}
	u, _ := url.Parse("https://www.tube.example/watch?v=1")
	cards, err := s.Resolve(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Kind != schema.KindVideo {
		t.Fatalf("bad cards: %+v", cards)
	}
	v := cards[0].Body.(*schema.VideoBody)
	if v.Channel != "Chan" || v.EmbedURL != "https://player.example/embed/1" {
		t.Fatalf("bad body: %+v", v)
	}
}

func TestUnknownHostPasses(t *testing.T) {
	s := New()
	u, _ := url.Parse("https://elsewhere.example/x")
	if _, err := s.Resolve(context.Background(), u); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled, got %v", err)
	}
}
