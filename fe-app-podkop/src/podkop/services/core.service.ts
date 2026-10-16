import { TabServiceInstance } from './tab.service';
import { store } from './store.service';
import { logger } from './logger.service';
import { PodkopLogWatcher } from './podkopLogWatcher.service';
import { PodkopShellMethods } from '../methods';

export function coreService() {
  TabServiceInstance.onChange((activeId, tabs) => {
    logger.info('[TAB]', activeId);
    store.set({
      tabService: {
        current: activeId || '',
        all: tabs.map((tab) => tab.id),
      },
    });
  });

  const watcher = PodkopLogWatcher.getInstance();

  // The notices ask only for the lines not seen yet. The cursor outlives a reload of the
  // page, as when LuCI goes to another page and back, for as long as the tab is open: a
  // line is raised once, not at every opening, and the answer stays small.
  const cursorKey = 'podkop-log-cursor';

  watcher.init(
    async () => {
      let cursor = '';
      try {
        cursor = sessionStorage.getItem(cursorKey) ?? '';
      } catch {
        // no storage: the lines are asked for from the start of the run
      }

      const logs = await PodkopShellMethods.checkLogsSince(cursor);

      if (logs.success && Array.isArray(logs.data?.lines)) {
        try {
          sessionStorage.setItem(
            cursorKey,
            `${logs.data.run}:${logs.data.next}`,
          );
        } catch {
          // no storage: the watcher still does not repeat a line while the page is open
        }

        return logs.data.lines.join('\n');
      }

      return '';
    },
    {
      intervalMs: 3000,
      onNewLog: (line) => {
        if (
          line.toLowerCase().includes('[error]') ||
          line.toLowerCase().includes('[fatal]')
        ) {
          ui.addNotification('Podkop Error', E('div', {}, [line]), 'error');
        }
      },
    },
  );

  watcher.start();
}
