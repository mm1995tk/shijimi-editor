package core

type fakeHost struct{ clip, url []string }

func (h *fakeHost) WriteClipboard(s string) { h.clip = append(h.clip, s) }

func (h *fakeHost) OpenURL(u string) { h.url = append(h.url, u) }
