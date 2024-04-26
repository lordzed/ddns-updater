# Ionos

## Configuration

### Example

```json
{
  "settings": [
    {
      "provider": "ionos",
      "domain": "domain.com",
      "host": "@",
      "api_key": "api_key",
      "ip_version": "ipv4",
      "ipv6_suffix": ""
    }
  ]
}
```

### Compulsory parameters

- `"domain"`
- `"host"` is your host and can be a subdomain or `"@"` or `"*"`
- `"api_key"` is your API key, obtained from [creating an API key](https://www.ionos.com/help/domains/configuring-your-ip-address/set-up-dynamic-dns-with-company-name/#c181598)

### Optional parameters

- `"ip_version"` can be `ipv4` (A records), or `ipv6` (AAAA records) or `ipv4 or ipv6` (update one of the two, depending on the public ip found). It defaults to `ipv4 or ipv6`.
- `"ipv6_suffix"` is the IPv6 interface identifiersuffix to use. It can be for example `0:0:0:0:72ad:8fbb:a54e:bedd/64`. If left empty, it defaults to no suffix and the raw public IPv6 address obtained is used in the record updating.