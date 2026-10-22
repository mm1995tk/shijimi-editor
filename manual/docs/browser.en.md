# External browser and port forwarding

shijimi has no built-in browser. Open web pages in the browser on your own machine. For a development server running on a remote VM, forward the port and view it in your local browser (see "Example: using it on a remote VM" below).

## Opening in an external browser

- `Space o` (Helix mode): open the URL under the cursor.
- `:open-url [URL]`: open the URL. Without an argument, it opens the URL under the cursor.

It uses `xdg-open` on Linux and `open` on macOS.

## Port forwarding

When you run `:browser forward <target port> [listen port]`, shijimi listens on `127.0.0.1` and forwards over TCP to `localhost:<target port>`.
Both HTTP and WebSocket pass through.

| Command | Action |
| --- | --- |
| `:browser forward <target port> [listen port]` | Start forwarding. If you omit the listen port, a free port is chosen |
| `:browser list` | List forwards |
| `:browser stop <listen port>` | Stop forwarding |

When you quit shijimi, all forwards stop.

### Example: using it on a remote VM

These are the steps to run shijimi on a VM and view a development server in your local browser.

1. In shijimi on the VM, run `:browser forward 3000`. Note the listen port that is shown.
2. On your local machine, run:

    ```sh
    ssh -L 8080:127.0.0.1:<listen port> vm
    ```

3. Open `http://localhost:8080` in your local browser.

Compared with using `ssh -L` directly, this has these advantages:

- You can fix the listen port.
- Servers that listen only on `0.0.0.0` or on IPv6 can also be reached via `127.0.0.1`.

If you need neither, use `ssh -L` directly.

### Listen address

```toml
[browser]
listen-host = "127.0.0.1"
```

The default is `127.0.0.1`. Change it to `0.0.0.0` only inside a trusted network.
