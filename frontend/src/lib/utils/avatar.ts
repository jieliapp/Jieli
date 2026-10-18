export const ALLOWED_AVATAR_MEDIA_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];

export function isSupportedAvatarMediaType(mediaType: string): boolean {
  return ALLOWED_AVATAR_MEDIA_TYPES.includes(mediaType);
}

export function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        resolve(reader.result);
      } else {
        reject(new Error('avatar file could not be read'));
      }
    };
    reader.onerror = () => reject(reader.error ?? new Error('avatar file could not be read'));
    reader.readAsDataURL(file);
  });
}
