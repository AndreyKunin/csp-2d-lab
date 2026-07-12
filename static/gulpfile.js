const gulp = require('gulp');
const browserify = require('browserify');
const source = require('vinyl-source-stream');
const buffer = require('vinyl-buffer');
const tsify = require('tsify');
const sourcemaps = require('gulp-sourcemaps');
const connect = require('gulp-connect');
const { createProxyMiddleware } = require('http-proxy-middleware');
const { deleteSync } = require('del');
const sass = require('gulp-sass')(require('sass'));
const watchify = require('watchify');

const paths = {
    src: {
        ts: 'src/**/*.{ts,tsx}',
        scss: 'src/styles/**/*.scss',
        html: 'src/index.html',
        assets: 'static/*.{ttf,svg,ico,js,png,jpg}',
        player: 'static/player/*.{svg,png,jpg}',
        lab: 'static/lab/*.{svg,png,jpg}'
    },
    dist: {
        root: 'dist/',
        js: 'dist/js/',
        css: 'dist/css/',
    },
    static: 'static/'
};

// Clean dist directory
function clean() {
    deleteSync([paths.dist.root]);
    return Promise.resolve();
}

// Compile TypeScript and bundle with browserify
function scripts() {
    return browserify({
        basedir: '.',
        debug: true,
        entries: ['src/index.tsx'],
        cache: {},
        packageCache: {},
        extensions: ['.tsx', '.ts', '.js']
    })
        .plugin(tsify, {
            target: 'ES2020',
            module: 'CommonJS',
            jsx: 'react-jsx'
        })
        .bundle()
        .pipe(source('bundle.js'))
        .pipe(buffer())
        .pipe(sourcemaps.init({ loadMaps: true }))
        .pipe(sourcemaps.write('./'))
        .pipe(gulp.dest(paths.dist.js));
}

// Compile SCSS to CSS
function styles() {
    return gulp.src('src/styles/main.scss')
        .pipe(sourcemaps.init())
        .pipe(sass({
            outputStyle: 'expanded',
            includePaths: ['src/styles/']
        }).on('error', sass.logError))
        .pipe(sourcemaps.write('./'))
        .pipe(gulp.dest(paths.dist.css));
}


// Copy HTML file
function html() {
    return gulp.src(paths.src.html)
        .pipe(gulp.dest(paths.dist.root));
}

// Copy existing static assets (math.js, fonts, etc.)
function assets() {
    return gulp.src(paths.src.assets)
        .pipe(gulp.dest(paths.dist.root));
}
function playerAssets() {
    return gulp.src(paths.src.player)
        .pipe(gulp.dest(paths.dist.root + 'player/'));
}
function labAssets() {
    return gulp.src(paths.src.lab)
        .pipe(gulp.dest(paths.dist.root + 'lab/'));
}

// Copy dist to static directory for Go server
function copyToStatic() {
    return gulp.src('dist/**/*')
        .pipe(gulp.dest(paths.static));
}

// Development server with proxy
function serve() {
    connect.server({
        root: 'dist',
        livereload: true,
        port: 3000,
        middleware: function(connect, opt) {
            return [
                createProxyMiddleware('/api', {
                    target: 'http://localhost:8080',
                    changeOrigin: true
                })
            ];
        }
    });
}

// Watch files for changes
function watch() {
    const watchOptions = {
        ignoreInitial: false,
        usePolling: true,
        interval: 1000,
        atomic: false
    };

    const watchedBrowserify = watchify(browserify({
        basedir: '.',
        debug: true,
        entries: ['src/index.tsx'],
        cache: {},
        packageCache: {},
        extensions: ['.tsx', '.ts', '.js']
    }).plugin(tsify, {
        target: 'ES2020',
        module: 'CommonJS',
        jsx: 'react-jsx'
    }));

    function rebundle() {

        return watchedBrowserify
            .bundle()
            .pipe(source('bundle.js'))
            .pipe(buffer())
            .pipe(sourcemaps.init({ loadMaps: true }))
            .pipe(sourcemaps.write('./'))
            .pipe(gulp.dest(paths.dist.js))
            .pipe(connect.reload());
    }

    watchedBrowserify.on('update', rebundle);
    watchedBrowserify.on('log', console.log);

    gulp.watch('src/styles/**/*.scss', watchOptions, gulp.series(styles, () => gulp.src(paths.dist.css + '**/*').pipe(connect.reload())));
    gulp.watch(paths.src.html, gulp.series(html, () => gulp.src(paths.dist.root + '*.html').pipe(connect.reload())));
    gulp.watch(paths.src.assets, gulp.series(assets, () => gulp.src(paths.dist.root + '**/*').pipe(connect.reload())));
    gulp.watch(paths.src.player, gulp.series(playerAssets, () => gulp.src(paths.dist.root + 'player/**/*').pipe(connect.reload())));
    gulp.watch(paths.src.lab, gulp.series(labAssets, () => gulp.src(paths.dist.root + 'lab/**/*').pipe(connect.reload())));

    return rebundle();
}

// Build tasks
const build = gulp.series(clean, gulp.parallel(scripts, styles, html, assets, playerAssets, labAssets));
const buildProduction = gulp.series(build, copyToStatic);
const dev = gulp.series(build, gulp.parallel(serve, watch));

// Export tasks
exports.clean = clean;
exports.scripts = scripts;
exports.styles = styles;
exports.html = html;
exports.assets = assets;
exports.playerAssets = playerAssets;
exports.labAssets = labAssets;
exports.serve = serve;
exports.watch = watch;
exports.build = buildProduction;
exports.dev = dev;
exports.default = build;