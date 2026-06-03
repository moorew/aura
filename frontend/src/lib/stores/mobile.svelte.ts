/** Reactive mobile breakpoint store (max-width: 767px) */
class MobileStore {
  value = $state(false);
  private mq: MediaQueryList | null = null;

  init() {
    if (typeof window === 'undefined') return;
    this.mq = window.matchMedia('(max-width: 767px)');
    this.value = this.mq.matches;
    this.mq.onchange = (e) => { this.value = e.matches; };
  }
}

export const mobile = new MobileStore();
