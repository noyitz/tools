// Diff rendering: color-coded headers and body diffs

const DiffRenderer = {
  renderHeaderMutations(containerId, mutations) {
    const container = document.getElementById(containerId);
    container.innerHTML = '';

    if (!mutations.set || !mutations.removed) {
      container.innerHTML = '<div class="placeholder">No mutations</div>';
      return;
    }

    const hasChanges = Object.keys(mutations.set).length > 0 || mutations.removed.length > 0;
    if (!hasChanges) {
      container.innerHTML = '<div class="placeholder">No header mutations (passthrough)</div>';
      return;
    }

    // Set headers
    for (const [key, value] of Object.entries(mutations.set)) {
      const row = document.createElement('div');
      row.className = 'result-header-row set';
      row.innerHTML = `
        <span class="badge set-badge">SET</span>
        <span class="key">${this.esc(key)}:</span>
        <span class="val">${this.esc(value)}</span>
      `;
      container.appendChild(row);
    }

    // Removed headers
    for (const key of mutations.removed) {
      const row = document.createElement('div');
      row.className = 'result-header-row removed';
      row.innerHTML = `
        <span class="badge removed-badge">DEL</span>
        <span class="key">${this.esc(key)}</span>
      `;
      container.appendChild(row);
    }
  },

  renderBodyResult(containerId, statusId, originalBody, result) {
    const container = document.getElementById(containerId);
    const statusEl = document.getElementById(statusId);

    if (!result.bodyMutation) {
      container.innerHTML = '<pre>' + this.esc(JSON.stringify(originalBody, null, 2)) + '</pre>';
      statusEl.textContent = 'unchanged';
      statusEl.className = 'body-status unchanged';
      return;
    }

    statusEl.textContent = 'mutated';
    statusEl.className = 'body-status mutated';

    if (result.bodyMutation.cleared) {
      container.innerHTML = '<div class="placeholder">Body cleared</div>';
      return;
    }

    if (result.bodyMutation.body) {
      const diffHtml = this.jsonDiff(originalBody, result.bodyMutation.body);
      container.innerHTML = '<pre>' + diffHtml + '</pre>';
    } else if (result.bodyMutation.raw) {
      container.innerHTML = '<pre>' + this.esc(result.bodyMutation.raw) + '</pre>';
    }
  },

  // Simple JSON diff: highlight added, removed, and changed keys
  jsonDiff(original, mutated) {
    const origStr = JSON.stringify(original, null, 2);
    const mutStr = JSON.stringify(mutated, null, 2);

    // Build key sets for top-level comparison
    const origKeys = new Set(Object.keys(original || {}));
    const mutKeys = new Set(Object.keys(mutated || {}));

    const lines = mutStr.split('\n');
    const result = [];

    for (const line of lines) {
      // Try to extract the key from this line
      const keyMatch = line.match(/^\s*"([^"]+)"\s*:/);
      if (keyMatch) {
        const key = keyMatch[1];
        if (!origKeys.has(key)) {
          result.push('<span class="diff-added">' + this.esc(line) + '</span>');
          continue;
        }
        // Check if value changed (simple string comparison of JSON values)
        const origVal = JSON.stringify(original[key]);
        const mutVal = JSON.stringify(mutated[key]);
        if (origVal !== mutVal) {
          result.push('<span class="diff-changed">' + this.esc(line) + '</span>');
          continue;
        }
      }
      result.push(this.esc(line));
    }

    return result.join('\n');
  },

  esc(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }
};
