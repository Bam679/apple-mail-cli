// Static Mail.app bridge. Never interpolate input into this file: all input
// arrives as decoded JSON in `a` (see osa.Prelude). There is deliberately no
// operation that sends, replies or forwards; `draft` only saves a draft.
const M = Application('Mail');
const LOCAL = 'On My Mac';

function ms(d) { return d ? d.getTime() : null; }
function isAccount(o) {
  // Only accounts have email addresses; class() is unreliable on specifiers
  // obtained through message.mailbox().
  try { o.emailAddresses(); return true; } catch (e) { /* not an account */ }
  try { return String(o.class()).toLowerCase().indexOf('account') >= 0; } catch (e) { return false; }
}

// pathOf walks containers up to the account (or the app for local boxes).
function pathOf(mb) {
  const parts = [mb.name()];
  let cur = mb;
  for (let d = 0; d < 16; d++) {
    let c;
    try { c = cur.container(); } catch (e) { return LOCAL + '/' + parts.join('/'); }
    if (isAccount(c)) return c.name() + '/' + parts.join('/');
    let n;
    try { n = c.name(); } catch (e) { return LOCAL + '/' + parts.join('/'); }
    if (!n || n === 'Mail') return LOCAL + '/' + parts.join('/');
    parts.unshift(n);
    cur = c;
  }
  return parts.join('/');
}

// allMailboxes returns [{path, account, spec}] for account and local boxes.
function allMailboxes() {
  const out = [];
  const accs = M.accounts;
  const names = accs.name();
  for (let k = 0; k < names.length; k++) {
    const mbs = accs[k].mailboxes;
    const n = mbs.name().length;
    for (let i = 0; i < n; i++) out.push({ path: pathOf(mbs[i]), account: names[k], spec: mbs[i] });
  }
  const local = M.mailboxes;
  const ln = local.name().length;
  for (let i = 0; i < ln; i++) out.push({ path: pathOf(local[i]), account: LOCAL, spec: local[i] });
  return out;
}

function mailboxByPath(path) {
  const all = allMailboxes();
  for (let i = 0; i < all.length; i++) if (all[i].path === path) return all[i];
  fail('not_found', 'no mailbox ' + path);
}

function messageById(id) {
  const n = Number(id);
  if (!isFinite(n) || n <= 0) fail('usage', 'invalid message id');
  try {
    const m = M.inbox.messages.byId(n);
    m.subject();
    return m;
  } catch (e) { /* not reachable through the unified inbox, scan boxes */ }
  const all = allMailboxes();
  for (let i = 0; i < all.length; i++) {
    try { const m = all[i].spec.messages.byId(n); m.subject(); return m; } catch (e) { /* next */ }
  }
  fail('not_found', 'no message with id ' + id);
}

function addresses(list) {
  try { return list.address(); } catch (e) { return []; }
}

function summary(m, path) {
  return { id: String(m.id()), subject: m.subject(), from: m.sender(), date_ms: ms(m.dateReceived()),
    read: m.readStatus(), flagged: m.flaggedStatus(), mailbox: path || pathOf(m.mailbox()) };
}

const OPS = {
  accounts: function () {
    const accs = M.accounts;
    const names = accs.name();
    return names.map(function (n, i) {
      let emails = [], enabled = true;
      try { emails = accs[i].emailAddresses(); } catch (e) { /* optional */ }
      try { enabled = accs[i].enabled(); } catch (e) { /* optional */ }
      return { name: n, emails: emails, enabled: enabled };
    });
  },
  mailboxes: function () {
    return allMailboxes().map(function (m) {
      let unread = 0;
      try { unread = m.spec.unreadCount(); } catch (e) { /* optional */ }
      return { path: m.path, account: m.account, unread: unread };
    });
  },
  inboxes: function () {
    const out = [];
    const ib = M.inbox.mailboxes;
    const n = ib.name().length;
    for (let i = 0; i < n; i++) out.push(pathOf(ib[i]));
    return out;
  },
  // messages lists newest-first across the given mailboxes with bulk reads
  // (never per-message loops over whole mailboxes), then resolves one page.
  messages: function (a) {
    const rows = [];
    const boxes = a.paths.map(mailboxByPath);
    const q = (a.query || '').toLowerCase();
    boxes.forEach(function (b, bi) {
      const msgs = b.spec.messages;
      const ids = msgs.id();
      if (!ids.length) return;
      const dates = msgs.dateReceived();
      const read = a.unread ? msgs.readStatus() : null;
      const flagged = a.flagged ? msgs.flaggedStatus() : null;
      let subj = null, from = null;
      if (q) { subj = msgs.subject(); from = a.subject_only ? null : msgs.sender(); }
      for (let i = 0; i < ids.length; i++) {
        if (read && read[i]) continue;
        if (flagged && !flagged[i]) continue;
        const t = ms(dates[i]);
        if (a.since_ms && t !== null && t < a.since_ms) continue;
        if (a.before_ms && t !== null && t >= a.before_ms) continue;
        if (q) {
          const hit = String(subj[i] || '').toLowerCase().indexOf(q) >= 0 ||
            (from && String(from[i] || '').toLowerCase().indexOf(q) >= 0);
          if (!hit) continue;
        }
        rows.push({ id: ids[i], t: t || 0, b: bi });
      }
    });
    rows.sort(function (x, y) { return y.t - x.t || y.id - x.id; });
    const page = rows.slice(a.offset, a.offset + a.limit).map(function (r) {
      return summary(boxes[r.b].spec.messages.byId(r.id), boxes[r.b].path);
    });
    return { total: rows.length, items: page };
  },
  get: function (a) {
    const m = messageById(a.id);
    const o = summary(m);
    o.message_id = m.messageId();
    o.to = addresses(m.toRecipients);
    o.cc = addresses(m.ccRecipients);
    o.date_sent_ms = ms(m.dateSent());
    o.size = m.messageSize();
    o.junk = m.junkMailStatus();
    o.attachments = [];
    try {
      const at = m.mailAttachments;
      const names = at.name();
      for (let i = 0; i < names.length; i++) {
        const x = { name: names[i], size: 0, mime: '' };
        try { x.size = at[i].fileSize(); } catch (e) { /* optional */ }
        try { x.mime = at[i].mimeType(); } catch (e) { /* mimeType() can throw */ }
        o.attachments.push(x);
      }
    } catch (e) { /* no attachments */ }
    if (a.source) {
      if (o.size > a.max_source) o.source_skipped = 'message larger than ' + a.max_source + ' bytes';
      else o.source = m.source();
    }
    return o;
  },
  mark: function (a) {
    const m = messageById(a.id);
    if (a.read !== null && a.read !== undefined) m.readStatus = a.read;
    if (a.flagged !== null && a.flagged !== undefined) m.flaggedStatus = a.flagged;
    return summary(m);
  },
  move: function (a) {
    const m = messageById(a.id);
    const rfc = m.messageId();
    const dst = mailboxByPath(a.to);
    M.move(m, { to: dst.spec });
    // Moving assigns a new id; find the message again by its Message-ID.
    try {
      const found = dst.spec.messages.whose({ messageId: rfc })();
      if (found.length) return summary(found[0], dst.path);
    } catch (e) { /* fall through */ }
    return { id: '', subject: '', from: '', date_ms: null, read: false, flagged: false, mailbox: dst.path };
  },
  delete: function (a) {
    const m = messageById(a.id);
    const s = summary(m);
    M.delete(m);
    return s;
  },
  draft: function (a) {
    const props = { subject: a.subject, content: a.body, visible: false };
    if (a.from) props.sender = a.from;
    const msg = M.OutgoingMessage(props);
    M.outgoingMessages.push(msg);
    (a.to || []).forEach(function (x) { msg.toRecipients.push(M.ToRecipient({ address: x })); });
    (a.cc || []).forEach(function (x) { msg.ccRecipients.push(M.CcRecipient({ address: x })); });
    (a.bcc || []).forEach(function (x) { msg.bccRecipients.push(M.BccRecipient({ address: x })); });
    msg.save();
    return { subject: a.subject, from: msg.sender(), to: a.to || [], saved: true };
  },
  mailbox_create: function (a) {
    M.make({ new: 'mailbox', withProperties: { name: a.name } });
    return { path: LOCAL + '/' + a.name, account: LOCAL, unread: 0 };
  },
};
