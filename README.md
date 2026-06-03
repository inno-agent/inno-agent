# inno-agent
# 📖 Инструкция по работе с проектом
 
## 1. Установка Lefthook
 
[Lefthook](https://github.com/evilmartians/lefthook) — менеджер git-хуков.
 
### Установка
 
**Homebrew, apt, winget** 
```bash
brew install lefthook
# или
sudo apt install lefthook
# или
winget install -e --id evilmartians.lefthook
``` 
Другой: https://lefthook.dev/install
### Активация хуков
 
```bash
lefthook install
```
 
Это зарегистрирует хуки из `lefthook.yml` в локальном `.git/hooks`.
