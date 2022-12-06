## HTTP/HTTPS proxy server, request repeater
индивидуальное задание по курсу "Безопасность интернет-приложений".


## Запуск
Чтобы проверить работу с HTTPS в curl или браузере необходимо добавить сертификат в список доверенных сертификатов.     
При отсутствии сертификата по пути caCrt и caKey в config/config.yml он будет сгенерирован програмно при запуске сервера и записан в соответсвующие файлы. Однако добавить в список доверенных CA необходимо вручную.
Для этого исполните локально на своём компьютере:
``` asm
$ sudo cp certs/repeater-proxy-ca.crt /usr/local/share/ca-certificates/
$ sudo update-ca-certificates
```
### Проверка сертификата
``` asm
$ sudo openssl verify certs/repeater-proxy-ca.crt
certs/repeater-proxy-ca.crt: OK
```
## Запуск в докере
``` asm
$ sudo docker build -t proxy .
$ sudo docker run -d -p 8080:8080 -p 8000:8000 -t proxy
```
## Запуск локально
``` asm
$ go run cmd/main.go
```
## Проверка работы прокси-сервера
``` asm
$ curl -i -x 127.0.0.1:8080 https://www.wikipedia.org/
$ curl -i -x 127.0.0.1:8080 http://mail.ru
$ curl -i 127.0.0.1:8000/requests
$ curl -i  127.0.0.1:8000/requests/1
$ curl -i  127.0.0.1:8000/repeat/1
```
