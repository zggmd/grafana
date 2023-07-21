import { IS_QIANKUN } from './constants';

declare let __webpack_public_path__: string;
if (IS_QIANKUN) {
  const devSubPath = process.env.NODE_ENV === 'development' ? 'public/build/' : '';
  __webpack_public_path__ = window.__INJECTED_PUBLIC_PATH_BY_QIANKUN__ + devSubPath;
}
