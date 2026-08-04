const { io } = require("socket.io-client");

const URL = process.argv[2] || "http://localhost:3002";
let passed = 0, failed = 0;
function check(name, cond) {
  if (cond) { passed++; console.log(`PASS  ${name}`); }
  else { failed++; console.log(`FAIL  ${name}`); }
}

async function delay(ms) { return new Promise(r => setTimeout(r, ms)); }

async function main() {
  // ---- 1. HTTP: GET / ----
  {
    const res = await fetch(URL + "/");
    const text = await res.text();
    check("GET / status 200", res.status === 200);
    check("GET / body exact", text.trim() === "Excalidraw collaboration server is up :)");
    check("GET / content-type text/html", (res.headers.get("content-type") || "").startsWith("text/html"));
  }
  // ---- 2. HTTP: static file ----
  {
    const res = await fetch(URL + "/test64.png");
    const buf = Buffer.from(await res.arrayBuffer());
    check("static /test64.png 200", res.status === 200);
    check("static /test64.png is PNG", buf[0] === 0x89 && buf[1] === 0x50);
  }
  // ---- 3. socket.io connection + init-room ----
  let initRoomCount = 0;
  const a = io(URL, { transports: ["websocket", "polling"], timeout: 5000 });
  const aEvents = { first: 0, newUser: null, roomChange: null, clientBcast: null, volBcast: null, followChange: null, unfollow: 0 };
  a.on("init-room", () => initRoomCount++);
  a.on("first-in-room", () => aEvents.first++);
  a.on("new-user", (id) => aEvents.newUser = id);
  a.on("room-user-change", (ids) => aEvents.roomChange = ids);
  a.on("client-broadcast", (data, iv) => aEvents.clientBcast = { data, iv });
  a.on("user-follow-room-change", (ids) => aEvents.followChange = ids);
  a.on("broadcast-unfollow", () => aEvents.unfollow++);
  a.on("connect", async () => {
    a.emit("join-room", "roomA");
  });
  await waitFor(() => aEvents.roomChange !== null && initRoomCount > 0, 5000);
  check("client connected with socket.id", typeof a.id === "string" && a.id.length > 0);
  check("init-room received", initRoomCount === 1);
  check("first-in-room received (solo)", aEvents.first === 1);
  check("room-user-change solo = [self]", aEvents.roomChange && aEvents.roomChange.length === 1 && aEvents.roomChange[0] === a.id);

  // ---- 4. second client joins: new-user + room-user-change ----
  const b = io(URL, { transports: ["websocket", "polling"], timeout: 5000 });
  const bEvents = { first: 0, newUser: null, roomChange: null, clientBcast: null };
  b.on("first-in-room", () => bEvents.first++);
  b.on("new-user", (id) => bEvents.newUser = id);
  b.on("room-user-change", (ids) => bEvents.roomChange = ids);
  b.on("client-broadcast", (data, iv) => bEvents.clientBcast = { data, iv });
  b.on("connect", async () => {
    b.emit("join-room", "roomA");
  });
  // a should get new-user with b's id, b should NOT get first-in-room
  await waitFor(() => aEvents.newUser !== null, 5000);
  check("a got new-user with b's id", aEvents.newUser === b.id);
  check("a got room-user-change with 2 ids", aEvents.roomChange && aEvents.roomChange.length === 2 && aEvents.roomChange.includes(a.id) && aEvents.roomChange.includes(b.id));
  await delay(300);
  check("b got room-user-change with 2 ids", bEvents.roomChange && bEvents.roomChange.length === 2);
  check("b did NOT get first-in-room", bEvents.first === 0);
  check("b did NOT get new-user (only sender broadcast)", bEvents.newUser === null);

  // ---- 5. server-broadcast binary relay ----
  aEvents.clientBcast = null;
  bEvents.clientBcast = null;
  const dataBuf = new Uint8Array([1, 2, 3, 4, 250]);
  const ivBuf = new Uint8Array([9, 8, 7]);
  a.emit("server-broadcast", "roomA", dataBuf, ivBuf);
  await waitFor(() => bEvents.clientBcast !== null, 3000);
  check("b received client-broadcast binary", bEvents.clientBcast !== null);
  if (bEvents.clientBcast) {
    const d = new Uint8Array(bEvents.clientBcast.data);
    const i = new Uint8Array(bEvents.clientBcast.iv);
    check("data bytes relayed exactly", d.length === 5 && d[0] === 1 && d[4] === 250);
    check("iv bytes relayed exactly", i.length === 3 && i[2] === 7);
  }
  await delay(200);
  check("a did NOT receive own broadcast", aEvents.clientBcast === null);

  // ---- 6. server-volatile-broadcast relay ----
  bEvents.clientBcast = null;
  const dataBuf2 = new Uint8Array([255, 254]);
  a.emit("server-volatile-broadcast", "roomA", dataBuf2, ivBuf);
  await waitFor(() => bEvents.clientBcast !== null, 3000);
  check("b received volatile client-broadcast", bEvents.clientBcast !== null);
  if (bEvents.clientBcast) {
    const d = new Uint8Array(bEvents.clientBcast.data);
    check("volatile data relayed", d.length === 2 && d[0] === 255 && d[1] === 254);
  }

  // ---- 7. user-follow ----
  aEvents.followChange = null;
  a.emit("user-follow", { userToFollow: { socketId: a.id, username: "alice" }, action: "FOLLOW" });
  await waitFor(() => aEvents.followChange !== null, 3000);
  check("follow -> user-follow-room-change [self]", aEvents.followChange && aEvents.followChange.length === 1 && aEvents.followChange[0] === a.id);

  // b follows a
  aEvents.followChange = null;
  b.emit("user-follow", { userToFollow: { socketId: a.id, username: "bob" }, action: "FOLLOW" });
  await waitFor(() => aEvents.followChange !== null, 3000);
  check("b follows a -> change list has 2", aEvents.followChange && aEvents.followChange.length === 2 && aEvents.followChange.includes(a.id) && aEvents.followChange.includes(b.id));

  // b unfollows a
  aEvents.followChange = null;
  b.emit("user-follow", { userToFollow: { socketId: a.id, username: "bob" }, action: "UNFOLLOW" });
  await waitFor(() => aEvents.followChange !== null, 3000);
  check("b unfollows a -> change list has 1", aEvents.followChange && aEvents.followChange.length === 1 && aEvents.followChange[0] === a.id);

  // ---- 8. disconnect: room-user-change + broadcast-unfollow ----
  // a follows b, so when a disconnects, follow@{b.id} becomes empty -> b gets broadcast-unfollow
  a.emit("user-follow", { userToFollow: { socketId: b.id, username: "alice" }, action: "FOLLOW" });
  await delay(200);
  const roomChangeOnB = new Promise(res => b.once("room-user-change", (ids) => res(ids)));
  const unfollowOnB = new Promise(res => b.once("broadcast-unfollow", () => res(true)));
  a.disconnect();  // a is in roomA (with b) and follow@{b.id}
  const remaining = await Promise.race([roomChangeOnB, delay(3000).then(() => null)]);
  check("after a disconnect, b got room-user-change with [b]", Array.isArray(remaining) && remaining.length === 1 && remaining[0] === b.id);
  const unfollowed = await Promise.race([unfollowOnB, delay(3000).then(() => false)]);
  check("follow@{b.id} emptied -> b got broadcast-unfollow", unfollowed === true);

  // CORS preflight check via fetch (OPTIONS)
  {
    const res = await fetch(URL + "/socket.io/?EIO=4&transport=polling", { method: "OPTIONS", headers: { "Origin": "http://example.com", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "content-type" } });
    check("OPTIONS preflight 2xx", res.status >= 200 && res.status < 300);
    const ao = res.headers.get("access-control-allow-origin");
    check("OPTIONS preflight allow-origin", ao === "http://example.com" || ao === "*");
    check("OPTIONS preflight allow-credentials", res.headers.get("access-control-allow-credentials") === "true");
  }

  console.log(`\n==== RESULT: ${passed} passed, ${failed} failed ====`);
  process.exit(failed === 0 ? 0 : 1);
}

function waitFor(cond, ms) {
  return new Promise((resolve, reject) => {
    const t0 = Date.now();
    const iv = setInterval(() => {
      if (cond()) { clearInterval(iv); resolve(); }
      else if (Date.now() - t0 > ms) { clearInterval(iv); reject(new Error("timeout waiting for condition")); }
    }, 30);
  });
}

main().catch(e => { console.error("TEST ERROR:", e.message); process.exit(1); });
