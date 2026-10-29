const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { validate, parseDotenv } = require('../dist/index.js');

test('parseDotenv parses unescaped values properly', () => {
  const tmpFile = path.join(os.tmpdir(), `test-env-${Date.now()}.env`);
  const content = `
PORT=8080
WIN_PATH="C:\\\\new"
GREETING="hello\\nworld"
EMPTY=
  `;
  fs.writeFileSync(tmpFile, content);

  try {
    const env = parseDotenv(tmpFile);
    assert.strictEqual(env.PORT, '8080');
    assert.strictEqual(env.WIN_PATH, 'C:\\new');
    assert.strictEqual(env.GREETING, 'hello\nworld');
    assert.strictEqual(env.EMPTY, '');
  } finally {
    fs.unlinkSync(tmpFile);
  }
});

test('validate enforces types and required fields', () => {
  const schema = {
    version: '1',
    vars: {
      PORT: { type: 'integer', required: true },
      DEBUG: { type: 'boolean', required: false, default: 'false' },
      EMAIL: { type: 'email', required: false },
    },
  };

  // Missing required PORT
  const badRes = validate(schema, { DEBUG: 'yes' });
  assert.strictEqual(badRes.hasErrors(), true);

  // Valid
  const goodRes = validate(schema, { PORT: '3000', DEBUG: 'yes', EMAIL: 'dev@example.com' });
  assert.strictEqual(goodRes.hasErrors(), false);
  assert.strictEqual(goodRes.valid, 3);
});
