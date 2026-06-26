# Secure Web Crawler (gRPC, Kubernetes, Istio mTLS)

Распределенный веб-краулер, построенный на базе микросервисной архитектуры с использованием **Go** и **gRPC**. 
Проект демонстрирует подходы безопасного межсервисного взаимодействия в кластере Kubernetes с применением паттерна **Service Mesh (Istio)** и архитектуры **Zero Trust (mTLS)**.

## 🏗 Архитектура

Система разделена на независимые микросервисы:

1. **Dispatcher (Оркестратор):** Управляет очередью ссылок и состоянием (visited URLs). Реализует паттерн *Worker Pool* и *Lock-free* синхронизацию через каналы (подход `Share memory by communicating`). Выступает gRPC-клиентом.
2. **Fetcher (Загрузчик):** Stateless-микросервис. Принимает gRPC-запросы, скачивает HTML-страницы и возвращает их содержимое.
3. **Parser (Парсер):** Stateless-микросервис. Принимает HTML, строит DOM-дерево, извлекает и нормализует ссылки (разрешение относительных путей), возвращает массив URL.

**Безопасность:** В коде микросервисов используется небезопасное соединение (`insecure.NewCredentials()`). Шифрованием трафика, ротацией ключей и взаимной аутентификацией (Mutual TLS) полностью управляет инфраструктурный слой — **Istio Envoy Proxy (Sidecar)**.

## 🛠 Стек технологий
* **Язык:** Golang 1.25
* **API:** gRPC, Protocol Buffers
* **Инфраструктура:** Docker, Kubernetes (Minikube)
* **Service Mesh:** Istio

---

## 🚀 Быстрый старт (Локальный запуск в Minikube)

### 1. Подготовка кластера и Service Mesh
Запустите локальный кластер и установите Istio:


```bash
# Запуск кластера
minikube start --driver=docker --memory=8192 --cpus=4

# Установка Istio
istioctl install --set profile=default -y

# Включение автоматической инжекции Envoy-прокси для нашего namespace
kubectl label namespace default istio-injection=enabled
```

### 2. Сборка Docker-образов
Чтобы кластер увидел локальные образы, переключаем контекст Docker на Minikube и собираем микросервисы:
```bash
eval $(minikube docker-env)

docker build --build-arg SERVICE_NAME=fetcher -t fetcher .
docker build --build-arg SERVICE_NAME=parser -t parser .
docker build --build-arg SERVICE_NAME=dispatcher -t dispatcher .
```

### 3. Деплой инфраструктуры
Применяем манифесты Kubernetes (Deployments, Services) и политику безопасности `PeerAuthentication`:
```bash
kubectl apply -f k8s-manifests.yaml
```
Убедитесь, что поды запущены и в каждом из них по **два** контейнера (приложение + envoy proxy):
```bash
kubectl get pods
# Ожидаемый результат: READY 2/2, STATUS Running
```

### 4. Запуск краулера
Запускаем Диспетчер как разовую задачу (Job/Pod):
```bash
kubectl run dispatcher --image=dispatcher --image-pull-policy=Never --restart=Never 
```
Смотрим логи работы распределенной системы в реальном времени:
```bash
kubectl logs -f dispatcher -c dispatcher
```

---

## 🔐 Демонстрация работы mTLS (Mutual TLS)

Этот проект настроен на **STRICT mTLS**. Любые попытки обратиться к микросервисам без валидного клиентского сертификата от внутреннего CA будут отклонены.

### 1. Проверка политик безопасности
Убедимся, что Istio применил политику STRICT для пода:
```bash
istioctl x describe pod $(kubectl get pod -l app=fetcher -o jsonpath='{.items[0].metadata.name}')
```
*В выводе должно быть указано: `Authentication: mTLS STRICT`.*

### 2. Извлечение и проверка динамических сертификатов
Envoy-прокси автоматически получают и ротируют сертификаты. Мы можем заглянуть в память Envoy и прочитать текущий сертификат Фетчера:
```bash
istioctl proxy-config secret $(kubectl get pod -l app=fetcher -o jsonpath='{.items[0].metadata.name}') -o json | jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | base64 --decode | openssl x509 -text -noout | grep -E "Issuer:|Validity|Subject Alternative Name" -A 2
```
*Вывод покажет, что сертификат выдан внутренним центром `O = cluster.local`, имеет короткий срок жизни (ротация) и идентифицирует под через стандарт SPIFFE (например, `spiffe://cluster.local/ns/default/sa/default`).*

### 3. Тест Zero-Trust (Попытка несанкционированного доступа)
Попробуем сделать plaintext HTTP/gRPC запрос к Фетчеру из пода, который не является частью Service Mesh (без Envoy и сертификатов):
```bash
kubectl run attacker --image=curlimages/curl --restart=Never -i --tty -- sh
# Внутри контейнера:
curl -v http://fetcher-service:10116
```
**Результат:** Соединение будет немедленно сброшено (`Connection reset by peer`). Envoy-прокси Фетчера отклоняет любой неаутентифицированный трафик, доказывая надежность mTLS.
```

