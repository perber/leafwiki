# LeafWiki Demo Setup (with nginx and HTTPS)

This guide demonstrates how to install **LeafWiki** on Ubuntu, configure **nginx** as a reverse proxy, and secure it with a free **Let's Encrypt** SSL certificate via **Certbot**.

---

## 1. Install LeafWiki

Run the installation script:

```bash
curl -sL https://raw.githubusercontent.com/perber/leafwiki/main/install.sh -o install.sh
chmod +x ./install.sh
sudo ./install.sh --arch amd64 --port 8080 --host 127.0.0.1
# Use --arch arm64 on ARM hosts (e.g. some cloud instances / Raspberry Pi).
```

Thanks to @Hugo-Galley for providing this installation script!

During installation, you’ll be prompted for:

- **JWT password:** choose a secure secret  
- **Admin password:** for the LeafWiki admin user  
- **Public read access (y/N):** enter `y` if you want guests to read without login  
- **Data directory (default /root/data):** press Enter for default or specify a path  

When complete, LeafWiki will be running as a systemd service:

```
Host: 127.0.0.1
Port: 8080
DataDirectory: /root/data
Status: active
```

---

## 2. Install nginx

```bash
sudo apt update
sudo apt install nginx -y
```

Enable and start nginx:

```bash
sudo systemctl enable nginx
sudo systemctl start nginx
```

---

## 3. Configure nginx as Reverse Proxy

Create a new site configuration file:

```bash
sudo nano /etc/nginx/sites-available/wiki.example.com.conf
```

Add the following content:

```nginx
server {
    listen 80;
    listen [::]:80;

    server_name wiki.example.com;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
```

Enable the site and reload nginx:

```bash
sudo ln -s /etc/nginx/sites-available/wiki.example.com.conf /etc/nginx/sites-enabled/wiki.example.com.conf
sudo nginx -t
sudo systemctl reload nginx
```

Now LeafWiki should be accessible at  
➡️ `http://wiki.example.com`

---

## 4. Install Certbot and Obtain an SSL Certificate

Install Certbot with nginx support:

```bash
sudo apt update
sudo apt install certbot python3-certbot-nginx -y
```

Obtain and install the certificate:

```bash
sudo certbot --nginx -d wiki.example.com
```

Follow the prompts:

- Enter a valid **email address**
- Agree to the **Terms of Service**
- (Optional) share your email with EFF
- Certbot will automatically edit the nginx config and enable HTTPS

---

## 5. Final nginx Configuration (with HTTPS)

After Certbot runs, your configuration at  
`/etc/nginx/sites-available/wiki.example.com.conf`  
should look like this:

```nginx
server {
    server_name wiki.example.com;
    client_max_body_size 50M;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }

    listen [::]:443 ssl ipv6only=on; # managed by Certbot
    listen 443 ssl;                  # managed by Certbot
    ssl_certificate     /etc/letsencrypt/live/wiki.example.com/fullchain.pem;  # managed by Certbot
    ssl_certificate_key /etc/letsencrypt/live/wiki.example.com/privkey.pem;    # managed by Certbot
    include             /etc/letsencrypt/options-ssl-nginx.conf;                # managed by Certbot
    ssl_dhparam         /etc/letsencrypt/ssl-dhparams.pem;                      # managed by Certbot
}

server {
    if ($host = wiki.example.com) {
        return 301 https://$host$request_uri;
    } # managed by Certbot

    listen 80;
    listen [::]:80;

    server_name wiki.example.com;
    return 404; # managed by Certbot
}
```

Now LeafWiki is available securely at:  
➡️ **https://wiki.example.com**

---

## 6. Serve LeafWiki under a URL prefix

To serve the wiki at `https://wiki.example.com/wiki/`, configure both LeafWiki and nginx to use the same prefix.

Set `LEAFWIKI_BASE_PATH=/wiki` in LeafWiki's environment (for the installation above, edit `/etc/leafwiki/.env`) and restart LeafWiki:

```bash
sudo systemctl restart leafwiki
```

For Docker, set the same environment variable or pass `--base-path=/wiki` to the container. The base path has a leading slash and no trailing slash.

In the HTTPS `server` block from step 5, replace `location /` with these two locations, keeping the TLS settings and `client_max_body_size`:

```nginx
location = /wiki {
    return 301 /wiki/;
}

location /wiki/ {
    proxy_pass         http://127.0.0.1:8080;
    proxy_http_version 1.1;

    proxy_set_header   Host              $host;
    proxy_set_header   X-Real-IP         $remote_addr;
    proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header   X-Forwarded-Proto $scheme;
}
```

Keep `proxy_pass` without a trailing slash or URI part. For example, `proxy_pass http://127.0.0.1:8080/;` would strip `/wiki/` from the forwarded request. LeafWiki expects that prefix when `--base-path=/wiki` is configured, so requests for assets such as `/wiki/static/...` would return 404.

Do not force `proxy_set_header Accept-Encoding gzip;`. Leave the browser's header unchanged; if desired, configure compression with nginx's `gzip` directives instead.

Validate and reload the configuration:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

Open `https://wiki.example.com/wiki/` and confirm that the page and its CSS/JavaScript assets load. Requests to `/wiki` should redirect to `/wiki/`.

---

## 7. Auto-Renew SSL Certificates

Certbot installs a renewal timer automatically.  
You can test it with:

```bash
sudo certbot renew --dry-run
```

---

✅ **Result:**  
- LeafWiki runs locally on port `8080`  
- nginx proxies requests from your domain  
- HTTPS is automatically managed by Let’s Encrypt  