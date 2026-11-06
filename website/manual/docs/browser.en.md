# External browser and port forwarding

In the terminal UI, shijimi-editor has no built-in browser: open web pages in the browser on your own machine (the GUI has a browser pane, see "Browser pane in the GUI" below). For a development server running on a remote VM, forward the port and view it in your local browser (see "Example: using it on a remote VM" below).

## Opening in an external browser

- `Space o` (Helix mode): open the URL under the cursor.
- `:open-url [URL]`: open the URL. Without an argument, it opens the URL under the cursor.

It uses `xdg-open` on Linux and `open` on macOS.


## Browser pane in the GUI

The GUI can show a page beside the terminals, so you do not have to leave the window. In the editor that runs in the primary terminal of a GUI workspace, `:open-url <URL>` (or `Space o` on a URL) does not call the browser of the machine. The GUI opens a browser pane next to the terminals. It works for pages on the internet and for a development server on the machine the workspace is on.

- Only `http` and `https` addresses open. The pane has back, forward, reload, an address bar, a button to open the page in the external browser (↗) and a close button (×). Type a host such as `localhost:3000` in the address bar and it is completed (`http://` for your own machine, `https://` for the rest).
- The page is shown in a web view of its own that lies over the pane, because most sites refuse to be shown in a frame. A web view lies over the GUI's own cards, so it is hidden while a card (for example an ssh question) is on the screen, and while the pane is not shown.
- The page cannot call the GUI's API. The GUI gives no permission to any address from the internet.
- **A development server on a remote machine** (`localhost`, `127.0.0.1`, `[::1]` or `0.0.0.0`): the GUI starts `ssh -N -L` to that machine for the pane and opens the page through it. The address bar keeps showing `localhost:<port>`. You may be asked the ssh password in the GUI. The forward stops when the pane closes. An address that is not the machine itself (such as `https://example.com`) is opened from your own machine.
- The pane is one of the terminals' cells (up to 6 cells in a workspace). It is not a tab. A workspace has one browser pane: opening another URL replaces it. Opening a URL shows the pane (even if the other panes are hidden) and moves the keys to the pane's frame, so the GUI's keys (`Ctrl+Shift+arrows` and so on) work; press the page to use the page. While the keys are in the page, click outside it to get the GUI's keys back.
- When you switch to another workspace the page is closed, and it is opened again from its first address when you come back.
- Links in a terminal do not open in the pane yet.

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

### Limits of the browser pane

- A link or a redirect inside a page of a remote machine that points to `http://localhost:<port>/…` as an absolute address is opened as `localhost` of your own machine, not through the forward. Type the address in the address bar instead.
- Passkeys (WebAuthn, for example the ones kept in 1Password) do not work in the pane: the web views of the GUI (WebKitGTK on Linux, WKWebView on macOS) do not offer them to arbitrary sites. Press the ↗ button in the pane to open the page again in the browser of your own machine, and use the passkey there. A page of a remote development server opens through the pane's ssh forward, so it works while the pane is open.
