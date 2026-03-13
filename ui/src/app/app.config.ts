import { ApplicationConfig, APP_INITIALIZER, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';

import { routes } from './app.routes';
import { httpErrorInterceptor } from './core/http-error.interceptor';
import { SiteConfigService } from './core/site-config.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes),
    provideHttpClient(withInterceptors([httpErrorInterceptor])),
    {
      provide: APP_INITIALIZER,
      useFactory: (siteConfig: SiteConfigService) => () => siteConfig.load(),
      deps: [SiteConfigService],
      multi: true,
    },
  ]
};
