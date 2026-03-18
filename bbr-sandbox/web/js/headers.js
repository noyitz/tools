// Headers editor: dynamic key-value rows

const HeadersEditor = {
  init(containerId, addBtnId) {
    this.container = document.getElementById(containerId);
    document.getElementById(addBtnId).addEventListener('click', () => this.addRow('', ''));
  },

  addRow(key, value) {
    const row = document.createElement('div');
    row.className = 'header-row';
    row.innerHTML = `
      <input type="text" class="header-key" placeholder="header-name" value="${this.esc(key)}">
      <input type="text" class="header-val" placeholder="value" value="${this.esc(value)}">
      <button class="btn-remove" title="Remove">&times;</button>
    `;
    row.querySelector('.btn-remove').addEventListener('click', () => row.remove());
    this.container.appendChild(row);
  },

  setHeaders(headers) {
    this.container.innerHTML = '';
    for (const [k, v] of Object.entries(headers)) {
      this.addRow(k, v);
    }
  },

  getHeaders() {
    const headers = {};
    for (const row of this.container.querySelectorAll('.header-row')) {
      const key = row.querySelector('.header-key').value.trim();
      const val = row.querySelector('.header-val').value;
      if (key) headers[key] = val;
    }
    return headers;
  },

  esc(s) {
    return s.replace(/"/g, '&quot;').replace(/</g, '&lt;');
  }
};
