# Thanos Federate Proxy

A proxy to convert `/federate` queries to `/v1/api/query` and respond in open metrics format.

The most common use case for this proxy is to be used as a side car with [Thanos](https://github.com/thanos-io/thanos), to provide `/federate` api for thanos (currently Thanos does not support it). So this way you can add Thanos as a federation source in another prometheus. Also Thanos does not support remote write (Note that being able to write metrics remotely from thanos to another prometheus is a different concept than thanos receiver component).

## Usage


### Docker

```bash
sudo docker run -p 9099:9099 ghcr.io/snapp-incubator/thanos-federate-proxy/image:latest -insecure-listen-address="0.0.0.0:9099"
```

### Binary releases

```bash
export VERSION=0.1.0
wget https://github.com/snapp-incubator/thanos-federate-proxy/releases/download/v${VERSION}/thanos-federate-proxy-${VERSION}.linux-amd64.tar.gz
tar xvzf thanos-federate-proxy-${VERSION}.linux-amd64.tar.gz thanos-federate-proxy-${VERSION}.linux-amd64/thanos-federate-proxy
```


### From source

```
git clone https://github.com/snapp-incubator/thanos-federate-proxy
go build
./thanos-federate-proxy <optional-extra-flags>
```



## Configuration


Flags:

```
  -insecure-listen-address string
        The address which proxy listens on (default "127.0.0.1:9099")
  -tlsSkipVerify
        Skip TLS Verfication (default false)
  -upstream string
        The upstream thanos URL (default "http://127.0.0.1:9090")
```

Sample k8s deployment (as a side car with thanos or prometheus):

```yml
containers:
  ...
- name: thanos-federate-proxy
  image: ghcr.io/snapp-incubator/thanos-federate-proxy/image:latest
  args:
  - -insecure-listen-address=0.0.0.0:9099
  - -upstream=http://127.0.0.1:9090
  ports:
  - containerPort: 9099
    name: fedproxy
    protocol: TCP
```


Sample prometheus config for federation:

```yml
scrape_configs:
- job_name: 'thanos-federate'
    scrape_timeout: 1m
    metrics_path: '/federate'
    params:
    'match[]':
    - 'up{namespace=~"perfix.*"}'
    static_configs:
    - targets:
        - 'thanos.svc.cluster:9099'
```

## Limitations

The following limitations will be addressed in future releases (see [Roadmap](#roadmap)):

- At the moment thanos-federate-proxy does not support multiple matcher queries and will only apply the first one.

- You can not pass empty matcher to prometheus for scraping all the metrics (see [this prometheus issue](https://github.com/prometheus/prometheus/issues/2162)). A workaround is to use the following matcher:

    ```
    'match[]':
    - '{__name__=~".+"}'
    ```

    Note that `{__name__=~".*"}` won't also work and you should use `".+"` instead of `".*"`.

## Roadmap

- [x] test federation
- [x] tlsSkipVerify flag
- [x] Dockerfile
- [x] github actions
- [ ] support multiple matchers
- [ ] support empty matchers
- [ ] support store-API for better performance
- [ ] metrics
- [ ] return error message instead of only logging it (??)
- [ ] remove space after comma in metrics (causing no issues)

## Metrics

(To be done)

| Metric                                              | Notes
|-----------------------------------------------------|------------------------------------
| thanosfederateproxy_scrape_duration_seconds_count   | Total number of scrape requests with response code
| thanosfederateproxy_scrape_duration_seconds_sum     | Duration of scrape requests with response code
| thanosfederateproxy_scrape_duration_seconds_bucket  | Count of scrape requests per bucket (for calculating percentile)

## Security

### Reporting security vulnerabilities

If you find a security vulnerability or any security related issues, please DO NOT file a public issue, instead send your report privately to cloud@snapp.cab. Security reports are greatly appreciated and we will publicly thank you for it.

## License

Apache-2.0 License, see [LICENSE](LICENSE).
