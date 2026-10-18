import { describe, expect, it } from 'vitest';
import en from '../lib/i18n/en.json';
import zh from '../lib/i18n/zh.json';
import { lookup } from '../lib/i18n';

describe('settings i18n', () => {
  it('exposes access-token labels in both locales', () => {
    expect(lookup(en, 'settings.copyToken')).toBe('Copy Token');
    expect(lookup(en, 'settings.rotateConfirm')).toBe(
      'Rotate the access token? The old token will stop working immediately.'
    );
    expect(lookup(zh, 'settings.copyToken')).toBe('复制密钥');
    expect(lookup(zh, 'settings.rotateConfirm')).toBe('确定要轮换访问令牌吗？旧令牌会立即失效。');
  });

  it('exposes account editing labels in both locales', () => {
    expect(lookup(en, 'settings.editDetails')).toBe('Edit Details');
    expect(lookup(en, 'settings.saveAccount')).toBe('Save changes');
    expect(lookup(en, 'settings.avatarUploadHint')).toBe('Click the avatar to upload a new image.');
    expect(lookup(zh, 'settings.editDetails')).toBe('编辑资料');
    expect(lookup(zh, 'settings.saveAccount')).toBe('保存更改');
    expect(lookup(zh, 'settings.avatarUploadHint')).toBe('点击头像上传新图片。');
  });
});
