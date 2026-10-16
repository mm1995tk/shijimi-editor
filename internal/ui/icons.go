package ui

import (
	"path/filepath"
	"strings"
)

// fileIcon is a Nerd Font glyph and its color.
type fileIcon struct {
	glyph string
	color string
}

var (
	iconFolder     = fileIcon{"", "#7aa2c8"}
	iconFolderOpen = fileIcon{"", "#7aa2c8"}
	iconDefault    = fileIcon{"", "#a0a0a0"}
)

var (
	icoGo       = fileIcon{"", "#00add8"}
	icoNix      = fileIcon{"", "#7ebae4"}
	icoMarkdown = fileIcon{"", "#519aba"}
	icoImage    = fileIcon{"", "#a074c4"}
	icoJSON     = fileIcon{"", "#cbcb41"}
	icoConfig   = fileIcon{"", "#9aa5b1"}
	icoShell    = fileIcon{"", "#89e051"}
	icoLock     = fileIcon{"", "#9aa5b1"}
	icoGit      = fileIcon{"", "#f1502f"}
	icoJS       = fileIcon{"", "#cbcb41"}
	icoTS       = fileIcon{"", "#519aba"}
	icoPython   = fileIcon{"", "#ffbc03"}
	icoRust     = fileIcon{"", "#dea584"}
	icoHTML     = fileIcon{"", "#e44d26"}
	icoCSS      = fileIcon{"", "#42a5f5"}
	icoText     = fileIcon{"", "#a0a0a0"}
	icoPDF      = fileIcon{"", "#e45649"}
	icoArchive  = fileIcon{"", "#c9a26d"}
	icoLicense  = fileIcon{"", "#d0bf41"}
)

// iconByName matches whole file names (lower-case) and wins over extensions.
var iconByName = map[string]fileIcon{
	"go.mod":         icoGo,
	"go.sum":         icoGo,
	"flake.lock":     icoNix,
	"license":        icoLicense,
	"makefile":       icoConfig,
	"dockerfile":     icoConfig,
	".gitignore":     icoGit,
	".gitattributes": icoGit,
	".gitmodules":    icoGit,
}

// iconByExt is keyed by lower-case extension without the dot.
var iconByExt = map[string]fileIcon{
	"go": icoGo, "nix": icoNix, "md": icoMarkdown, "markdown": icoMarkdown,
	"png": icoImage, "jpg": icoImage, "jpeg": icoImage, "gif": icoImage,
	"webp": icoImage, "bmp": icoImage, "svg": icoImage, "ico": icoImage,
	"json": icoJSON, "toml": icoConfig, "yaml": icoConfig, "yml": icoConfig,
	"sh": icoShell, "bash": icoShell, "zsh": icoShell,
	"lock": icoLock, "js": icoJS, "mjs": icoJS, "ts": icoTS, "tsx": icoTS,
	"py": icoPython, "rs": icoRust, "html": icoHTML, "css": icoCSS,
	"txt": icoText, "pdf": icoPDF,
	"zip": icoArchive, "tar": icoArchive, "gz": icoArchive,
}

// iconFor picks the icon for a tree node.
func iconFor(n *node) fileIcon {
	if n.isDir {
		if n.expanded {
			return iconFolderOpen
		}
		return iconFolder
	}
	name := strings.ToLower(n.name)
	if ic, ok := iconByName[name]; ok {
		return ic
	}
	if ic, ok := iconByExt[strings.TrimPrefix(filepath.Ext(name), ".")]; ok {
		return ic
	}
	return iconDefault
}
