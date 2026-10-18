import type { ParamMatcher } from '@sveltejs/kit';

/** Only the non-default locale gets a URL prefix; English stays at the root. */
export const match: ParamMatcher = (param) => param === 'zh';
