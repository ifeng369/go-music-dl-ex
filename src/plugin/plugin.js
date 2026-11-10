/**
 * MusicFree plugin for go-music-dl-ex.
 *
 * Install this single file in MusicFree, then set "API 地址" to the address of
 * the go-music-dl-ex server (for example http://192.168.1.10:8080).
 *
 * The plugin intentionally has no build step and no npm dependencies. It uses
 * the HTTP client that MusicFree injects (axios, fetch or needle).
 */

const DEFAULT_BASE = "http://127.0.0.1:8080";

let authToken = "";
let authUser = "";
let loginPromise = null;

function getCredentials() {
  try {
    if (typeof env !== "undefined" && typeof env.getUserVariables === "function") {
      const vars = env.getUserVariables() || {};
      return {
        username: String(vars.username || "").trim(),
        password: String(vars.password || ""),
      };
    }
  } catch (e) {
    // ignore
  }
  return { username: "", password: "" };
}

function authHeaders() {
  return authToken ? { Authorization: "Bearer " + authToken } : {};
}

function getBase() {
  try {
    if (typeof env !== "undefined" && typeof env.getUserVariables === "function") {
      const vars = env.getUserVariables() || {};
      const base = vars.apiBase || vars.api_base || vars.baseUrl;
      if (base) return String(base).replace(/\/+$/, "");
    }
  } catch (e) {
    // ignore and use the default
  }
  return DEFAULT_BASE;
}

function buildQuery(params) {
  return Object.keys(params || {})
    .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== "")
    .map((key) => encodeURIComponent(key) + "=" + encodeURIComponent(params[key]))
    .join("&");
}

function httpPost(path, body) {
  const url = getBase() + path;
  const payload = body || {};
  const headers = { Accept: "application/json", "Content-Type": "application/json" };
  if (typeof axios !== "undefined") {
    return axios.post(url, payload, { timeout: 20000, headers }).then((res) => res.data);
  }
  if (typeof fetch !== "undefined") {
    return fetch(url, { method: "POST", headers, body: JSON.stringify(payload) }).then((res) => {
      if (!res.ok) throw new Error("HTTP " + res.status + " " + url);
      return res.json();
    });
  }
  if (typeof needle !== "undefined") {
    return new Promise((resolve, reject) => {
      needle.post(url, payload, { json: true, timeout: 20000, headers }, (err, res, data) => {
        if (err) return reject(err);
        if (res && res.statusCode >= 400) return reject(new Error("HTTP " + res.statusCode));
        resolve(data);
      });
    });
  }
  return Promise.reject(new Error("no HTTP client available in this MusicFree build"));
}

function ensureLogin() {
  const { username, password } = getCredentials();
  if (!username || !password) {
    return Promise.reject(new Error("请在插件设置中填写用户名和密码"));
  }
  if (authToken && authUser === username) {
    return Promise.resolve(authToken);
  }
  if (loginPromise) return loginPromise;
  loginPromise = httpPost("/api/auth/login", { username, password })
    .then((data) => {
      if (!data || !data.token) throw new Error("登录失败");
      authToken = data.token;
      authUser = username;
      loginPromise = null;
      return authToken;
    })
    .catch((err) => {
      loginPromise = null;
      authToken = "";
      authUser = "";
      throw err;
    });
  return loginPromise;
}

function httpGet(path, params) {
  return ensureLogin().then(() => {
    const url = getBase() + path + (params ? "?" + buildQuery(params) : "");
    const headers = Object.assign({ Accept: "application/json" }, authHeaders());

    if (typeof axios !== "undefined") {
      return axios.get(url, { timeout: 20000, headers }).then((res) => res.data);
    }
    if (typeof fetch !== "undefined") {
      return fetch(url, { headers }).then((res) => {
        if (!res.ok) throw new Error("HTTP " + res.status + " " + url);
        return res.json();
      });
    }
    if (typeof needle !== "undefined") {
      return new Promise((resolve, reject) => {
        needle.get(url, { json: true, timeout: 20000, headers }, (err, res, body) => {
          if (err) return reject(err);
          if (res && res.statusCode >= 400) return reject(new Error("HTTP " + res.statusCode));
          resolve(body);
        });
      });
    }
    return Promise.reject(new Error("no HTTP client available in this MusicFree build"));
  });
}

function splitId(item) {
  const rawId = String(item.sourceId || item.id || "");
  if (item.source) {
    return { source: item.source, id: rawId.indexOf(item.source + ":") === 0 ? rawId.slice(item.source.length + 1) : rawId };
  }
  const idx = rawId.indexOf(":");
  if (idx > 0) return { source: rawId.slice(0, idx), id: rawId.slice(idx + 1) };
  return { source: "", id: rawId };
}

function itemParams(item) {
  const pair = splitId(item);
  const extra = item.extra ? JSON.stringify(item.extra) : "";
  return {
    source: pair.source,
    id: pair.id,
    title: item.title,
    artist: item.artist,
    album: item.album,
    duration: item.duration,
    ext: item.ext,
    cover: item.cover || item.artwork,
    extra,
  };
}

function normalizeMusic(item) {
  if (!item) return item;
  return Object.assign({}, item, {
    platform: item.platform || "go-music-dl",
    id: item.id || (item.source && item.sourceId ? item.source + ":" + item.sourceId : item.sourceId),
  });
}

function normalizeSheet(item, type) {
  if (!item) return item;
  return Object.assign({}, item, {
    platform: item.platform || "go-music-dl",
    id: item.id || (item.source && item.sourceId ? item.source + ":" + item.sourceId : item.sourceId),
    type: item.type || type || "sheet",
  });
}

const plugin = {
  platform: "Go Music DL",
  version: "1.0.0",
  author: "go-music-dl-ex",
  srcUrl: "https://github.com/guohuiyuan/go-music-dl",
  cacheControl: "no-cache",
  primaryKey: ["id"],
  supportedSearchType: ["music", "sheet", "album", "artist"],
  hints: {
    importMusicSheet: [
      "粘贴网易云 / QQ音乐 / 酷狗 / 酷我 / 咪咕 / 汽水等平台的歌单、专辑链接",
      "需要先在设置里把 API 地址改成 go-music-dl-ex 的地址",
    ],
    importMusicItem: ["粘贴单曲分享链接，例如 https://music.163.com/song?id=..."],
  },
  userVariables: [
    {
      key: "apiBase",
      name: "API 地址",
      description: "go-music-dl-ex 服务地址，例如 http://192.168.1.10:8080",
      defaultValue: DEFAULT_BASE,
    },
    {
      key: "username",
      name: "用户名",
      description: "登录音乐库的用户名；插件只会访问该账号自己的收藏、歌单与本地音乐",
      defaultValue: "admin",
    },
    {
      key: "password",
      name: "密码",
      description: "登录音乐库的密码（默认管理员为 admin123，请在设置中及时修改）",
      defaultValue: "admin123",
    },
  ],

  async search(query, page, type) {
    const searchType = type === "sheet" ? "sheet" : type === "album" ? "album" : type === "artist" ? "artist" : "music";
    const data = await httpGet("/api/search", {
      q: query,
      type: searchType,
      page: page || 1,
      limit: 30,
    });
    const items = (data && data.items) || [];
    const isEnd = Boolean(data && data.isEnd) || items.length < 30;
    if (searchType === "music") return { isEnd, musicList: items.map(normalizeMusic) };
    if (searchType === "artist") {
      return {
        isEnd,
        artistList: items.map((item) =>
          Object.assign({}, item, {
            platform: item.platform || "go-music-dl",
            avatar: item.avatar || item.artwork || "",
          })
        ),
      };
    }
    if (searchType === "album") {
      return { isEnd, albumList: items.map((item) => normalizeSheet(item, "album")) };
    }
    return { isEnd, sheetList: items.map((item) => normalizeSheet(item, "sheet")) };
  },

  async getMediaSource(mediaItem) {
    const params = itemParams(mediaItem);
    await ensureLogin();
    const query = buildQuery(Object.assign({}, params, { access_token: authToken }));
    return {
      url: getBase() + "/api/stream?" + query,
      headers: authToken ? { Authorization: "Bearer " + authToken } : {},
    };
  },

  async getMusicInfo(musicItem) {
    const params = itemParams(musicItem);
    const data = await httpGet("/api/song", params);
    return normalizeMusic(Object.assign({}, musicItem, data));
  },

  async getLyric(musicItem) {
    const params = itemParams(musicItem);
    const data = await httpGet("/api/lyric", params);
    return { rawLrc: (data && (data.rawLrc || data.lrc)) || "" };
  },

  async getAlbumInfo(albumItem, page) {
    const params = itemParams(albumItem);
    params.page = page || 1;
    params.limit = 500;
    const data = await httpGet("/api/album", params);
    return {
      isEnd: !!(data && data.isEnd),
      musicList: ((data && data.musicList) || []).map(normalizeMusic),
      albumItem: normalizeSheet(data && data.sheetItem, "album"),
    };
  },

  async getMusicSheetInfo(sheetItem, page) {
    const params = itemParams(sheetItem);
    params.page = page || 1;
    params.limit = 500;
    const data = await httpGet("/api/playlist", params);
    return {
      isEnd: !!(data && data.isEnd),
      musicList: ((data && data.musicList) || []).map(normalizeMusic),
      sheetItem: normalizeSheet(data && data.sheetItem, "playlist"),
    };
  },

  async getTopLists() {
    const data = await httpGet("/api/toplists");
    const groups = (data && data.groups) || [];
    if (groups.length > 0) {
      return groups.map((group) => ({
        title: group.title || group.source,
        data: (group.data || []).map((item) => normalizeSheet(item, "toplist")),
      }));
    }
    return [{ title: "排行榜", data: ((data && data.items) || []).map((item) => normalizeSheet(item, "toplist")) }];
  },

  async getTopListDetail(topListItem, page) {
    const params = itemParams(topListItem);
    params.page = page || 1;
    params.limit = 500;
    const data = await httpGet("/api/toplist/detail", params);
    return {
      isEnd: !!(data && data.isEnd),
      topListItem: normalizeSheet((data && data.sheetItem) || topListItem, "toplist"),
      musicList: ((data && data.musicList) || []).map(normalizeMusic),
    };
  },

  async getRecommendSheetTags() {
    const data = await httpGet("/api/categories");
    const bySource = {};
    ((data && data.items) || []).forEach((category) => {
      const source = category.source || "netease";
      if (!bySource[source]) bySource[source] = [];
      bySource[source].push({
        id: category.id || category.name,
        title: category.name,
        source,
        group: category.group,
      });
    });
    const groups = Object.keys(bySource).map((source) => ({
      title: source,
      data: bySource[source],
    }));
    return {
      pinned: { id: "recommend", title: "推荐", source: "" },
      data: groups,
    };
  },

  async getRecommendSheetsByTag(tag, page) {
    if (!tag || tag.id === "recommend") {
      const data = await httpGet("/api/recommend", { source: tag && tag.source ? tag.source : "all" });
      return { isEnd: true, data: ((data && data.items) || []).map((item) => normalizeSheet(item, "playlist")) };
    }
    const data = await httpGet("/api/category/sheets", {
      source: tag.source || "netease",
      category: tag.id,
      page: page || 1,
      limit: 30,
    });
    return {
      isEnd: !!(data && data.isEnd),
      data: ((data && data.items) || []).map((item) => normalizeSheet(item, "playlist")),
    };
  },

  async getArtistWorks(artistItem, page, type) {
    const data = await httpGet("/api/artist/works", {
      name: artistItem.name || artistItem.id,
      source: artistItem.source || "",
      page: page || 1,
      limit: 100,
    });
    return {
      isEnd: !!(data && data.isEnd),
      musicList: ((data && data.items) || []).map(normalizeMusic),
    };
  },

  async importMusicSheet(urlLike) {
    const data = await httpGet("/api/import", { url: urlLike, type: "sheet" });
    if (data && data.sheet) {
      const sheet = normalizeSheet(data.sheet, data.type || "playlist");
      if (data.musicList && data.musicList.length) {
        sheet.musicList = data.musicList.map(normalizeMusic);
      }
      return sheet;
    }
    if (data && data.music) {
      return {
        platform: "go-music-dl",
        id: data.music.id,
        title: data.music.title || data.music.album || "导入结果",
        artwork: data.music.artwork,
        musicList: [normalizeMusic(data.music)],
      };
    }
    return null;
  },

  async importMusicItem(urlLike) {
    const data = await httpGet("/api/import", { url: urlLike, type: "music" });
    if (data && data.music) return normalizeMusic(data.music);
    if (data && data.sheet && data.musicList && data.musicList.length) return normalizeMusic(data.musicList[0]);
    return null;
  },
};

module.exports = plugin;
