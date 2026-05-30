# LeafWiki Demo Setup (with nginx and HTTPS)

This guide demonstrates how to install **LeafWiki** on Ubuntu, configure **nginx** as a reverse proxy, and secure it with a free **Let's Encrypt** SSL certificate via **Certbot**.

---

## 1. Install LeafWiki

Run the installation script:

```bash
curl -sL https://raw.githubusercontent.com/perber/leafwiki/main/install.sh -o install.sh
chmod +x ./install.sh
sudo ./install.sh
```

Thanks to @Hugo-Galley for providing this installation script!

During installation, you’ll be prompted for:

- **Architecture (amd64/arm64):** choose the release for your machine
- **JWT password:** choose a secure secret
- **Admin password:** for the LeafWiki admin user
- **Host:** use `127.0.0.1` when nginx is the public entrypoint
- **Port:** use `8080` to match the nginx example below
- **Public read access (y/N):** enter `y` if you want guests to read without login
- **Data directory (default `$PWD/data`):** press Enter for default or specify a path
- **Root directory (default `<data-directory>/root`):** optional managed markdown directory

Changing the root directory does not migrate existing markdown pages. For an existing install, move or copy the old `<data-directory>/root` content before starting LeafWiki with a new root directory.

When complete, LeafWiki will be running as a systemd service:

```
Host: 127.0.0.1
Port: 8080
DataDirectory: <current-directory>/data
RootDirectory: <current-directory>/data/root
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
sudo nano /etc/nginx/sites-available/demo.leafwiki.com.conf
```

Add the following content:

```nginx
server {
    listen 80;
    listen [::]:80;

    server_name demo.leafwiki.com;

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
sudo ln -s /etc/nginx/sites-available/demo.leafwiki.com.conf /etc/nginx/sites-enabled/demo.leafwiki.com.conf
sudo nginx -t
sudo systemctl reload nginx
```

Now LeafWiki should be accessible at
➡️ `http://demo.leafwiki.com`

---

## 4. Install Certbot and Obtain an SSL Certificate

Install Certbot with nginx support:

```bash
sudo apt update
sudo apt install certbot python3-certbot-nginx -y
```

Obtain and install the certificate:

```bash
sudo certbot --nginx -d demo.leafwiki.com
```

Follow the prompts:

- Enter a valid **email address**
- Agree to the **Terms of Service**
- (Optional) share your email with EFF
- Certbot will automatically edit the nginx config and enable HTTPS

---

## 5. Final nginx Configuration (with HTTPS)

After Certbot runs, your configuration at
`/etc/nginx/sites-available/demo.leafwiki.com.conf`
should look like this:

```nginx
server {
    server_name demo.leafwiki.com;
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
    ssl_certificate     /etc/letsencrypt/live/demo.leafwiki.com/fullchain.pem;  # managed by Certbot
    ssl_certificate_key /etc/letsencrypt/live/demo.leafwiki.com/privkey.pem;    # managed by Certbot
    include             /etc/letsencrypt/options-ssl-nginx.conf;                # managed by Certbot
    ssl_dhparam         /etc/letsencrypt/ssl-dhparams.pem;                      # managed by Certbot
}

server {
    if ($host = demo.leafwiki.com) {
        return 301 https://$host$request_uri;
    } # managed by Certbot

    listen 80;
    listen [::]:80;

    server_name demo.leafwiki.com;
    return 404; # managed by Certbot
}
```

Now LeafWiki is available securely at:
➡️ **https://demo.leafwiki.com**

---

## 6. Auto-Renew SSL Certificates

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
