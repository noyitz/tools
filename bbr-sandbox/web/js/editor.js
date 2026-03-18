// JSON editor with validation

const JsonEditor = {
  init(textareaId, errorId) {
    this.textarea = document.getElementById(textareaId);
    this.errorEl = document.getElementById(errorId);

    this.textarea.addEventListener('input', () => this.validate());
    this.textarea.addEventListener('keydown', (e) => {
      // Tab inserts spaces
      if (e.key === 'Tab') {
        e.preventDefault();
        const start = this.textarea.selectionStart;
        this.textarea.value = this.textarea.value.substring(0, start) + '  ' + this.textarea.value.substring(this.textarea.selectionEnd);
        this.textarea.selectionStart = this.textarea.selectionEnd = start + 2;
      }
    });
  },

  validate() {
    try {
      if (this.textarea.value.trim()) {
        JSON.parse(this.textarea.value);
      }
      this.textarea.classList.remove('invalid');
      this.errorEl.textContent = '';
      return true;
    } catch (e) {
      this.textarea.classList.add('invalid');
      this.errorEl.textContent = e.message;
      return false;
    }
  },

  getValue() {
    try {
      return JSON.parse(this.textarea.value);
    } catch {
      return null;
    }
  },

  setValue(obj) {
    this.textarea.value = JSON.stringify(obj, null, 2);
    this.validate();
  },

  prettify() {
    const val = this.getValue();
    if (val !== null) {
      this.textarea.value = JSON.stringify(val, null, 2);
      this.validate();
    }
  }
};
