import ReactDOM from 'react-dom';

import './public-path';

import app from '../app';

type OnGlobalStateChangeCallback = (state: Record<string, object>, prevState: Record<string, object>) => void;
type props = {
  onGlobalStateChange: (callback: OnGlobalStateChangeCallback, fireImmediately?: boolean) => void;
  setLoading: (loading: boolean) => void;
  container: {
    querySelector: (selector: string) => undefined;
  };
};

export async function bootstrap() {
  console.log('grafana:bootstraped');
}

export async function mount(props: props) {
  props.onGlobalStateChange(async () => {
    props.setLoading(false);
  }, true);
  console.log('grafana:mount');
  app.init(props.container.querySelector('#reactRoot'));
}

export async function unmount(props: props) {
  ReactDOM.unmountComponentAtNode(
    // @ts-ignore
    props.container ? props.container.querySelector('#reactRoot') : document.querySelector('#root')
  );
}
