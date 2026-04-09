package main

import "sync"

type Lang string

const (
LangZhCN Lang = "zh-CN"
LangEnUS Lang = "en-US"
)

type I18n struct {
mu   sync.RWMutex
lang Lang
}

func NewI18n() *I18n {
return &I18n{lang: LangZhCN}
}

func (i *I18n) SetLang(lang Lang) {
i.mu.Lock()
defer i.mu.Unlock()
if lang == LangZhCN || lang == LangEnUS {
i.lang = lang
}
}

func (i *I18n) GetLang() Lang {
i.mu.RLock()
defer i.mu.RUnlock()
return i.lang
}
