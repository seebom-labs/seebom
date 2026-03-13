# Ui

This project was generated using [Angular CLI](https://github.com/angular/angular-cli) version 21.2.1.

## Development server

To start a local development server, run:

```bash
ng serve
```

Once the server is running, open your browser and navigate to `http://localhost:4200/`. The application will automatically reload whenever you modify any of the source files.

## Code scaffolding

Angular CLI includes powerful code scaffolding tools. To generate a new component, run:

```bash
ng generate component component-name
```

For a complete list of available schematics (such as `components`, `directives`, or `pipes`), run:

```bash
ng generate --help
```

## Building

To build the project run:

```bash
ng build
```

This will compile your project and store the build artifacts in the `dist/` directory. By default, the production build optimizes your application for performance and speed.

## Running unit tests

To execute unit tests with the [Vitest](https://vitest.dev/) test runner, use the following command:

```bash
ng test
```

## Running end-to-end tests

For end-to-end (e2e) testing, run:

```bash
ng e2e
```

Angular CLI does not come with an end-to-end testing framework by default. You can choose one that suits your needs.

## Additional Resources

For more information on using the Angular CLI, including detailed command references, visit the [Angular CLI Overview and Command Reference](https://angular.dev/tools/cli) page.

## Customisation

### Custom Theme (CSS)

Override any CSS variable without rebuilding. Mount a `custom-theme.css` into the nginx webroot:

```bash
CUSTOM_THEME=./my-theme.css docker compose up -d --force-recreate ui
```

See `src/assets/custom-theme.example.css` for all available variables.

### Site Configuration (Texts & Branding)

All UI text content is configurable via `public/ui-config.json` (loaded at runtime, no rebuild needed):

| Key | Description |
|-----|-------------|
| `brandName` | Navbar brand text |
| `pageTitle` | Browser tab title |
| `dashboard.title` | Dashboard heading |
| `dashboard.subtitle` | Dashboard subheading |
| `dashboard.description` | Description banner (HTML supported) |
| `dashboard.disclaimer` | Disclaimer text (HTML supported) |
| `footer.enabled` | Show/hide footer |
| `footer.text` | Footer text content |

All fields are optional — missing keys fall back to built-in defaults.

For Docker Compose, use the `UI_CONFIG` env variable to mount a custom file. For Kubernetes, enable `ui.siteConfig` in Helm values.

